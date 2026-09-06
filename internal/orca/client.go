package orca

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Runner func(context.Context, string, ...string) ([]byte, error)

type Client struct {
	Binary  string
	Timeout time.Duration
	Run     Runner
}

// Command never invokes a shell. Stderr is kept separate from the JSON stream.
func Command(ctx context.Context, binary string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.WaitDelay = time.Second
	return cmd.Output()
}

type envelope struct {
	OK     *bool           `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  struct {
		Code string `json:"code"`
	} `json:"error"`
}

func (c *Client) read(ctx context.Context, target any, args ...string) error {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	run := c.Run
	if run == nil {
		run = Command
	}
	data, runErr := run(ctx, c.Binary, args...)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &Error{"timeout"}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(runErr, exec.ErrNotFound) || errors.Is(runErr, os.ErrNotExist) || errors.Is(runErr, os.ErrPermission) {
		return &Error{"orca_unavailable"}
	}
	var env envelope
	parseErr := json.Unmarshal(data, &env)
	if parseErr == nil && env.OK != nil && !*env.OK {
		// Unknown CLI codes are upstream failures, never silently classified as no sessions.
		switch env.Error.Code {
		case "app_not_running", "runtime_not_ready", "runtime_unavailable", "runtime_unreachable", "connection_refused", "not_connected":
			return &Error{"orca_unavailable"}
		}
		return &Error{"upstream_error"}
	}
	if runErr != nil {
		return &Error{"upstream_error"}
	}
	if parseErr != nil || env.OK == nil || len(env.Result) == 0 || bytes.Equal(bytes.TrimSpace(env.Result), []byte("null")) {
		return &Error{"invalid_response"}
	}
	if err := json.Unmarshal(env.Result, target); err != nil {
		return &Error{"invalid_response"}
	}
	return nil
}

func (c *Client) Worktrees(ctx context.Context) (WorktreeResult, error) {
	var result WorktreeResult
	if err := c.read(ctx, &result, "worktree", "ps", "--json"); err != nil {
		return result, err
	}
	if result.Worktrees == nil {
		return result, &Error{"invalid_response"}
	}
	seen := map[string]bool{}
	for _, w := range result.Worktrees {
		if strings.TrimSpace(w.WorktreeID) == "" || seen[w.WorktreeID] {
			return result, &Error{"invalid_response"}
		}
		seen[w.WorktreeID] = true
	}
	return result, nil
}

func (c *Client) Terminals(ctx context.Context) (TerminalResult, error) {
	var result TerminalResult
	if err := c.read(ctx, &result, "terminal", "list", "--json"); err != nil {
		return result, err
	}
	if result.Terminals == nil {
		return result, &Error{"invalid_response"}
	}
	seen := map[string]bool{}
	for _, t := range result.Terminals {
		if strings.TrimSpace(t.Handle) == "" || seen[t.Handle] {
			return result, &Error{"invalid_response"}
		}
		seen[t.Handle] = true
	}
	return result, nil
}

func (c *Client) Sessions(ctx context.Context) (Snapshot, error) {
	// Independent reads run concurrently; wait for both before returning.
	var w WorktreeResult
	var werr error
	done := make(chan struct{})
	go func() { defer close(done); w, werr = c.Worktrees(ctx) }()
	t, terr := c.Terminals(ctx)
	<-done
	if werr != nil {
		return Snapshot{}, werr
	}
	if terr != nil {
		return Snapshot{}, terr
	}
	return Normalize(w, t), nil
}

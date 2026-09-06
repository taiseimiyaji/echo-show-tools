package orca

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

func TestFixtures(t *testing.T) {
	c := Client{Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		name := "worktrees"
		want := []string{"worktree", "ps", "--json"}
		if args[0] == "terminal" {
			name = "terminals"
			want = []string{"terminal", "list", "--json"}
		}
		if !reflect.DeepEqual(args, want) {
			t.Errorf("unexpected command: %v", args)
		}
		return fixtures.ReadFile("testdata/" + name + ".json")
	}}
	s, err := c.Sessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sessions) != 3 || s.Meta.Partial {
		t.Fatalf("unexpected snapshot: %+v", s)
	}
	if s.Sessions[0].Agent != "codex" || s.Sessions[0].Status != "running" || s.Sessions[1].Status != "done" {
		t.Fatalf("agent join failed: %+v", s.Sessions)
	}
	if s.Sessions[2].Agent != "unknown" || s.Sessions[2].LastMessage != "" || s.Sessions[2].LastActivityAt != nil {
		t.Fatal("shell inherited agent data")
	}
}

func TestDecodeFailures(t *testing.T) {
	for _, tc := range []struct {
		name, data, code string
		err              error
	}{
		{"malformed", "{", "invalid_response", nil},
		{"missing ok", `{"result":{"worktrees":[]}}`, "invalid_response", nil},
		{"missing result", `{"ok":true}`, "invalid_response", nil},
		{"null result", `{"ok":true,"result":null}`, "invalid_response", nil},
		{"missing array", `{"ok":true,"result":{}}`, "invalid_response", nil},
		{"null array", `{"ok":true,"result":{"worktrees":null}}`, "invalid_response", nil},
		{"missing id", `{"ok":true,"result":{"worktrees":[{}]}}`, "invalid_response", nil},
		{"failed envelope", `{"ok":false,"error":{"code":"anything","message":"private output"}}`, "upstream_error", nil},
		{"not running", `{"ok":false,"error":{"code":"app_not_running"}}`, "orca_unavailable", errors.New("exit 1")},
		{"nonzero", `{"ok":true,"result":{"worktrees":[]}}`, "upstream_error", errors.New("private stderr")},
		{"missing cli", "", "orca_unavailable", exec.ErrNotFound},
		{"missing absolute cli", "", "orca_unavailable", os.ErrNotExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Client{Run: func(context.Context, string, ...string) ([]byte, error) { return []byte(tc.data), tc.err }}
			_, err := c.Worktrees(context.Background())
			var got *Error
			if !errors.As(err, &got) || got.Code != tc.code {
				t.Fatalf("got %v want %s", err, tc.code)
			}
		})
	}
	c := Client{Run: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"ok":true,"result":{"worktrees":[],"futureField":42}}`), nil
	}}
	if _, err := c.Worktrees(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTimeout(t *testing.T) {
	c := Client{Timeout: 10 * time.Millisecond, Run: func(ctx context.Context, _ string, _ ...string) ([]byte, error) { <-ctx.Done(); return nil, ctx.Err() }}
	_, err := c.Worktrees(context.Background())
	if err == nil || err.Error() != "timeout" {
		t.Fatalf("got %v", err)
	}
}

// Run the test binary as a real fake CLI to exercise process behavior without Orca.
func TestFakeCLIProcess(t *testing.T) {
	if os.Getenv("AGENT_DECK_HELPER") == "stderr" {
		_, _ = os.Stderr.WriteString("Electron diagnostic: private detail\n")
		_, _ = os.Stdout.WriteString(`{"ok":true,"result":{"worktrees":[]}}`)
		os.Exit(0)
	}
	if os.Getenv("AGENT_DECK_HELPER") == "wait" {
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_DECK_HELPER", "stderr")
	data, err := Command(context.Background(), binary, "-test.run=^TestFakeCLIProcess$")
	if err != nil || string(data) != `{"ok":true,"result":{"worktrees":[]}}` {
		t.Fatalf("stdout polluted: %q %v", data, err)
	}
	t.Setenv("AGENT_DECK_HELPER", "wait")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := Command(ctx, binary, "-test.run=^TestFakeCLIProcess$"); err == nil {
		t.Fatal("process was not interrupted")
	}
}

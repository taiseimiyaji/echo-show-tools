package orca

import (
	"context"
	"embed"
	"time"
)

// These synthetic fixtures preserve the observed CLI structure without real user data.
//
//go:embed testdata/*.json
var fixtures embed.FS

type Demo struct{}

func (Demo) Sessions(ctx context.Context) (Snapshot, error) {
	c := Client{Binary: "demo", Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		name := "worktrees"
		if args[0] == "terminal" {
			name = "terminals"
		}
		return fixtures.ReadFile("testdata/" + name + ".json")
	}}
	s, err := c.Sessions(ctx)
	s.Meta.Demo = true
	for i := range s.Sessions {
		at := time.Now().Add(-time.Duration(i+1) * time.Minute).UnixMilli()
		s.Sessions[i].LastActivityAt = &at
	}
	return s, err
}

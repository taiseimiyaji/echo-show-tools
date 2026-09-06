package orca

import "testing"

func ptr[T any](v T) *T { return &v }

func inputs() (WorktreeResult, TerminalResult) {
	w := WorktreeResult{Worktrees: []Worktree{{WorktreeID: "w", Repo: "repo", WorkspaceStatus: "in-progress", Agents: []Agent{{PaneKey: "tab:leaf", AgentType: "codex", State: "working", LastAssistantMessage: "message"}}}}, TotalCount: ptr(1), Truncated: ptr(false)}
	ts := TerminalResult{Terminals: []Terminal{{Handle: "term", WorktreeID: "w", TabID: "tab", LeafID: "leaf", ExecutionHostID: "local", Connected: ptr(true), AgentIdentity: "claude"}}, TotalCount: ptr(1), Truncated: ptr(false), HostScope: &HostScope{HostIDs: []string{"local"}, OmittedHostIDs: []string{}}}
	return w, ts
}

func TestStatusAndAssociation(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		mutate                 func(*WorktreeResult, *TerminalResult)
		agent, status, message string
	}{
		{"structured wins", func(w *WorktreeResult, ts *TerminalResult) {}, "codex", "running", "message"},
		{"done", func(w *WorktreeResult, ts *TerminalResult) { w.Worktrees[0].Agents[0].State = "done" }, "codex", "done", "message"},
		{"interrupt wins done", func(w *WorktreeResult, ts *TerminalResult) {
			w.Worktrees[0].Agents[0].State = "done"
			w.Worktrees[0].Agents[0].Interrupted = true
		}, "codex", "interrupted", "message"},
		{"offline wins interrupt", func(w *WorktreeResult, ts *TerminalResult) {
			w.Worktrees[0].Agents[0].Interrupted = true
			ts.Terminals[0].Connected = ptr(false)
		}, "codex", "offline", "message"},
		{"missing connection is not offline", func(w *WorktreeResult, ts *TerminalResult) { ts.Terminals[0].Connected = nil }, "codex", "running", "message"},
		{"unknown state", func(w *WorktreeResult, ts *TerminalResult) { w.Worktrees[0].Agents[0].State = "future" }, "codex", "active", "message"},
		{"unknown workspace state", func(w *WorktreeResult, ts *TerminalResult) {
			w.Worktrees[0].Agents[0].State = "future"
			w.Worktrees[0].WorkspaceStatus = "todo"
		}, "codex", "unknown", "message"},
		{"different pane", func(w *WorktreeResult, ts *TerminalResult) { ts.Terminals[0].LeafID = "shell" }, "claude", "active", ""},
		{"ambiguous pane", func(w *WorktreeResult, ts *TerminalResult) {
			w.Worktrees[0].Agents = append(w.Worktrees[0].Agents, w.Worktrees[0].Agents[0])
		}, "claude", "active", ""},
		{"empty pane", func(w *WorktreeResult, ts *TerminalResult) {
			ts.Terminals[0].TabID = ""
			ts.Terminals[0].LeafID = ""
			w.Worktrees[0].Agents[0].PaneKey = ":"
		}, "claude", "active", ""},
		{"unknown terminal", func(w *WorktreeResult, ts *TerminalResult) {
			ts.Terminals[0].LeafID = "shell"
			ts.Terminals[0].AgentIdentity = ""
		}, "unknown", "active", ""},
		{"unjoined", func(w *WorktreeResult, ts *TerminalResult) { ts.Terminals[0].WorktreeID = "other" }, "claude", "unknown", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, ts := inputs()
			tc.mutate(&w, &ts)
			s := Normalize(w, ts).Sessions[0]
			if s.Agent != tc.agent || s.Status != tc.status || s.LastMessage != tc.message {
				t.Fatalf("got %+v", s)
			}
		})
	}
}

func TestPartialAndOrdering(t *testing.T) {
	for _, mutate := range []func(*WorktreeResult, *TerminalResult){
		func(w *WorktreeResult, ts *TerminalResult) { w.Truncated = ptr(true) },
		func(w *WorktreeResult, ts *TerminalResult) { ts.TotalCount = ptr(2) },
		func(w *WorktreeResult, ts *TerminalResult) { ts.HostScope = nil },
		func(w *WorktreeResult, ts *TerminalResult) { ts.HostScope.OmittedHostIDs = []string{"remote"} },
		func(w *WorktreeResult, ts *TerminalResult) { ts.Terminals[0].ExecutionHostID = "" },
		func(w *WorktreeResult, ts *TerminalResult) { ts.Terminals[0].WorktreeID = "unknown" },
	} {
		w, ts := inputs()
		mutate(&w, &ts)
		if !Normalize(w, ts).Meta.Partial {
			t.Fatal("partial result hidden")
		}
	}
	w, ts := inputs()
	ts.Terminals = []Terminal{{Handle: "z", LastOutputAt: ptr(int64(10))}, {Handle: "b"}, {Handle: "a"}, {Handle: "c", LastOutputAt: ptr(int64(-1))}}
	s := Normalize(w, ts).Sessions
	if s[0].ID != "z" || s[1].ID != "a" || s[2].ID != "b" || s[3].ID != "c" || s[3].LastActivityAt != nil {
		t.Fatalf("unstable order: %+v", s)
	}
}

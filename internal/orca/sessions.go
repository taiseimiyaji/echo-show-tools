package orca

import (
	"sort"
	"strings"
	"time"
)

func Normalize(worktrees WorktreeResult, terminals TerminalResult) Snapshot {
	out := Snapshot{Sessions: []Session{}, Meta: Meta{FetchedAt: time.Now().UnixMilli()}}
	out.Meta.Partial = incomplete(worktrees.Truncated, worktrees.TotalCount, len(worktrees.Worktrees)) || incomplete(terminals.Truncated, terminals.TotalCount, len(terminals.Terminals))
	hosts := map[string]bool{}
	if s := terminals.HostScope; s == nil {
		out.Meta.Partial = true
	} else {
		for _, id := range s.HostIDs {
			hosts[id] = true
		}
		if s.HostIDs == nil || s.OmittedHostIDs == nil || len(s.OmittedHostIDs) > 0 {
			out.Meta.Partial = true
		}
	}
	byID := map[string]Worktree{}
	for _, w := range worktrees.Worktrees {
		byID[w.WorktreeID] = w
	}
	for _, t := range terminals.Terminals {
		if strings.HasPrefix(t.WorktreeID, "ephemeral-") {
			continue
		}
		if t.ExecutionHostID == "" || !hosts[t.ExecutionHostID] {
			out.Meta.Partial = true
		}
		w, found := byID[t.WorktreeID]
		if !found {
			out.Meta.Partial = true
		}
		s := Session{ID: t.Handle, Repo: w.Repo, Title: t.Title, Agent: "unknown", Status: "unknown", Connected: t.Connected, Writable: t.Writable, LastActivityAt: validTime(t.LastOutputAt)}
		if s.Title == "" {
			s.Title = w.DisplayName
		}
		if s.Title == "" {
			s.Title = "Terminal"
		}
		if t.AgentIdentity != "" {
			s.Agent = t.AgentIdentity
		}
		// Observed in Orca 1.4.197: paneKey = tabId + ":" + leafId.
		// Require a unique match; do not project workspace agents onto other panes.
		var matched *Agent
		matches := 0
		if t.TabID != "" && t.LeafID != "" {
			for i := range w.Agents {
				if w.Agents[i].PaneKey == t.TabID+":"+t.LeafID {
					matched = &w.Agents[i]
					matches++
				}
			}
		}
		if matches != 1 {
			matched = nil
		}
		if w.WorkspaceStatus == "in-progress" {
			s.Status = "active"
		}
		if matched != nil {
			if matched.AgentType != "" {
				s.Agent = matched.AgentType
			}
			s.LastMessage = matched.LastAssistantMessage
			if at := validTime(matched.UpdatedAt); at != nil && (s.LastActivityAt == nil || *at > *s.LastActivityAt) {
				s.LastActivityAt = at
			}
			switch matched.State {
			case "working":
				s.Status = "running"
			case "done":
				s.Status = "done"
			}
			if matched.Interrupted {
				s.Status = "interrupted"
			}
		}
		if t.Connected != nil && !*t.Connected {
			s.Status = "offline"
		}
		out.Sessions = append(out.Sessions, s)
	}
	sort.Slice(out.Sessions, func(i, j int) bool {
		a, b := out.Sessions[i], out.Sessions[j]
		if a.LastActivityAt == nil && b.LastActivityAt != nil {
			return false
		}
		if a.LastActivityAt != nil && b.LastActivityAt == nil {
			return true
		}
		if a.LastActivityAt != nil && b.LastActivityAt != nil && *a.LastActivityAt != *b.LastActivityAt {
			return *a.LastActivityAt > *b.LastActivityAt
		}
		return a.ID < b.ID
	})
	return out
}

func incomplete(truncated *bool, total *int, count int) bool {
	return truncated == nil || *truncated || total == nil || *total != count
}
func validTime(v *int64) *int64 {
	if v == nil || *v <= 0 {
		return nil
	}
	return v
}

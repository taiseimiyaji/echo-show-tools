package orca

type Agent struct {
	PaneKey              string `json:"paneKey"`
	AgentType            string `json:"agentType"`
	State                string `json:"state"`
	Interrupted          bool   `json:"interrupted"`
	LastAssistantMessage string `json:"lastAssistantMessage"`
	UpdatedAt            *int64 `json:"updatedAt"`
}

type Worktree struct {
	WorktreeID      string  `json:"worktreeId"`
	Repo            string  `json:"repo"`
	DisplayName     string  `json:"displayName"`
	WorkspaceStatus string  `json:"workspaceStatus"`
	Agents          []Agent `json:"agents"`
}

type Terminal struct {
	Handle          string `json:"handle"`
	WorktreeID      string `json:"worktreeId"`
	TabID           string `json:"tabId"`
	LeafID          string `json:"leafId"`
	Title           string `json:"title"`
	AgentIdentity   string `json:"agentIdentity"`
	Connected       *bool  `json:"connected"`
	Writable        *bool  `json:"writable"`
	LastOutputAt    *int64 `json:"lastOutputAt"`
	ExecutionHostID string `json:"executionHostId"`
}

type WorktreeResult struct {
	Worktrees  []Worktree `json:"worktrees"`
	TotalCount *int       `json:"totalCount"`
	Truncated  *bool      `json:"truncated"`
}

type HostScope struct {
	HostIDs        []string `json:"hostIds"`
	OmittedHostIDs []string `json:"omittedHostIds"`
}

type TerminalResult struct {
	Terminals  []Terminal `json:"terminals"`
	TotalCount *int       `json:"totalCount"`
	Truncated  *bool      `json:"truncated"`
	HostScope  *HostScope `json:"hostScope"`
}

type Session struct {
	ID             string `json:"id"`
	Repo           string `json:"repo"`
	Title          string `json:"title"`
	Agent          string `json:"agent"`
	Status         string `json:"status"`
	Connected      *bool  `json:"connected"`
	Writable       *bool  `json:"writable"`
	LastMessage    string `json:"lastMessage"`
	LastActivityAt *int64 `json:"lastActivityAt"`
}

type Meta struct {
	Partial   bool  `json:"partial"`
	Demo      bool  `json:"demo"`
	FetchedAt int64 `json:"fetchedAt"`
}

type Snapshot struct {
	Sessions []Session `json:"sessions"`
	Meta     Meta      `json:"meta"`
}

// Error codes deliberately contain no CLI output or local environment details.
type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

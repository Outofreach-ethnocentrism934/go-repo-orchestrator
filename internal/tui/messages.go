package tui

import (
	"context"

	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

type branchesLoadedMsg struct {
	requestID    int
	actionKey    string
	actionID     int
	repoName     string
	rb           model.RepoBranches
	err          error
	startup      bool
	jiraResolved int
	syncNote     string
}

// startupLogMsg позволяет фоновым задачам отправлять записи в лог загрузки TUI.
type startupLogMsg struct{ text string }

type playwrightStartupCompletedMsg struct{ err error }

type scriptGeneratedMsg struct {
	result model.ScriptResult
	err    error
}

type initialLoadMsg struct{}

type repoStatLoadedMsg struct {
	actionKey string
	actionID  int
	repoName  string
	stat      model.RepoStat
	err       error
	startup   bool
}

type checkoutCompletedMsg struct {
	actionKey string
	actionID  int
	repoName  string
	err       error
}

type localCopyCompletedMsg struct {
	actionKey string
	actionID  int
	repoName  string
	branch    string
	err       error
}

type repoFetchPullCompletedMsg struct {
	actionKey string
	actionID  int
	repoName  string
	err       error
}

type actionCancelRef struct {
	id     int
	cancel context.CancelFunc
}

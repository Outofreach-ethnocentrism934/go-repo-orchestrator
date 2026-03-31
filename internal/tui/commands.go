package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/agelxnash/go-repo-orchestrator/internal/config"
	"github.com/agelxnash/go-repo-orchestrator/internal/model"
	"github.com/agelxnash/go-repo-orchestrator/internal/usecase"
)

// loadRepoBranchesCmd запускает загрузку веток репозитория и отправляет поэтапные события в лог.
func loadRepoBranchesCmd(ctx context.Context, cleaner *usecase.Cleaner, repo config.RepoConfig, requestID int, startup bool, actionKey string, actionID int) tea.Cmd {
	if !startup {
		return func() tea.Msg {
			rb, err := cleaner.LoadRepoBranches(ctx, repo)
			return branchesLoadedMsg{requestID: requestID, actionKey: actionKey, actionID: actionID, repoName: repo.Name, rb: rb, err: err, startup: false}
		}
	}

	stageGit := func() tea.Msg {
		return startupLogMsg{fmt.Sprintf("[GIT] %s: получение веток...", repo.Name)}
	}
	loadAndReport := func() tea.Msg {
		rb, err := cleaner.LoadRepoBranches(ctx, repo)
		if err != nil {
			return branchesLoadedMsg{requestID: requestID, actionKey: actionKey, actionID: actionID, repoName: repo.Name, rb: rb, err: err, startup: true}
		}
		jiraResolved := 0
		for _, b := range rb.Branches {
			if b.JiraKey != "-" && b.JiraKey != "" && b.JiraStatus != "-" && b.JiraStatus != "" {
				jiraResolved++
			}
		}
		syncNote := ""
		if rb.SyncWarning != "" {
			syncNote = " [из кэша]"
		}
		return branchesLoadedMsg{
			requestID:    requestID,
			actionKey:    actionKey,
			actionID:     actionID,
			repoName:     repo.Name,
			rb:           rb,
			err:          nil,
			startup:      true,
			jiraResolved: jiraResolved,
			syncNote:     syncNote,
		}
	}
	return tea.Batch(stageGit, loadAndReport)
}

func loadRepoStatCmd(ctx context.Context, cleaner *usecase.Cleaner, repo config.RepoConfig, startup bool, actionKey string, actionID int) tea.Cmd {
	return func() tea.Msg {
		stat, err := cleaner.LoadRepoStat(ctx, repo)
		return repoStatLoadedMsg{actionKey: actionKey, actionID: actionID, repoName: repo.Name, stat: stat, err: err, startup: startup}
	}
}

func generateScriptCmd(cleaner *usecase.Cleaner, repo config.RepoConfig, repoPath string, branches []model.BranchInfo, format model.ScriptFormat) tea.Cmd {
	return func() tea.Msg {
		result, err := cleaner.GenerateDeleteScript(repo, repoPath, branches, format)
		return scriptGeneratedMsg{result: result, err: err}
	}
}

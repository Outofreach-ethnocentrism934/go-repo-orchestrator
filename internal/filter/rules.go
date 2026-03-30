package filter

import (
	"github.com/agelxnash/go-repo-orchestrator/internal/config"
	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

// Evaluate применяет базовые правила фильтрации ветки и возвращает допуск и причину.
func Evaluate(repo config.RepoConfig, branch model.BranchInfo, currentBranch, defaultBranch string) (bool, string) {
	if branch.Name == currentBranch {
		return false, "current branch"
	}

	if defaultBranch != "" && branch.Name == defaultBranch {
		return false, "default branch"
	}

	if reason, ok := repo.ProtectedReason(branch.Name); ok {
		return false, reason
	}

	return true, "eligible"
}

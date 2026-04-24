package usecase

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/agelxnash/go-repo-orchestrator/internal/config"
	"github.com/agelxnash/go-repo-orchestrator/internal/jira"
	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

type RepoRelease struct {
	Group   string
	Version jira.ReleaseVersion
}

type ReleaseAutocheckResult struct {
	Group                string
	ReleaseID            string
	IssueKeysTotal       int
	BranchMatches        int
	BranchCandidates     int
	BranchSkippedProtect int
	BranchSkippedNoJira  int
}

func (c *Cleaner) ListRepoReleasedFixVersions(ctx context.Context, repo config.RepoConfig, branches []model.BranchInfo) ([]RepoRelease, error) {
	groups := collectMappedJiraGroups(repo, branches)
	if len(groups) == 0 {
		return nil, nil
	}

	releases := make([]RepoRelease, 0, 8)
	var firstErr error
	for _, group := range groups {
		versions, err := c.rel.ListReleasedFixVersions(ctx, group)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		for _, version := range versions {
			if strings.TrimSpace(version.ID) == "" {
				continue
			}
			releases = append(releases, RepoRelease{Group: group, Version: version})
		}
	}

	if len(releases) == 0 && firstErr != nil {
		return nil, fmt.Errorf("jira releases недоступны: %w", firstErr)
	}

	slices.SortStableFunc(releases, func(a, b RepoRelease) int {
		if a.Version.ReleaseDate != b.Version.ReleaseDate {
			if a.Version.ReleaseDate > b.Version.ReleaseDate {
				return -1
			}
			return 1
		}
		if a.Group != b.Group {
			return strings.Compare(strings.ToLower(a.Group), strings.ToLower(b.Group))
		}
		return strings.Compare(strings.ToLower(a.Version.Name), strings.ToLower(b.Version.Name))
	})

	return releases, nil
}

func (c *Cleaner) BuildReleaseAutocheckCandidates(ctx context.Context, repo config.RepoConfig, branches []model.BranchInfo, group, releaseID string) (ReleaseAutocheckResult, []model.BranchInfo, error) {
	group = strings.TrimSpace(group)
	releaseID = strings.TrimSpace(releaseID)

	result := ReleaseAutocheckResult{Group: group, ReleaseID: releaseID}
	if group == "" || releaseID == "" {
		return result, nil, nil
	}

	issueKeys, err := c.rel.ListDoneIssueKeysByRelease(ctx, group, releaseID)
	if err != nil {
		return result, nil, fmt.Errorf("получить Jira задачи релиза: %w", err)
	}

	issueSet := make(map[string]struct{}, len(issueKeys))
	for _, issueKey := range issueKeys {
		normalized := strings.ToUpper(strings.TrimSpace(issueKey))
		if normalized == "" {
			continue
		}
		issueSet[normalized] = struct{}{}
	}
	result.IssueKeysTotal = len(issueSet)

	candidates := make([]model.BranchInfo, 0)
	seen := make(map[string]struct{})
	for _, branch := range branches {
		jiraMatch, ok, _ := repo.ExtractJiraMatchDetailed(branch.Name)
		if !ok {
			result.BranchSkippedNoJira++
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(jiraMatch.Group), group) {
			continue
		}

		jiraKey := strings.ToUpper(strings.TrimSpace(jiraMatch.Key))
		if jiraKey == "" {
			result.BranchSkippedNoJira++
			continue
		}
		if _, found := issueSet[jiraKey]; !found {
			continue
		}

		result.BranchMatches++
		if branch.Protected {
			result.BranchSkippedProtect++
			continue
		}

		selectionKey := strings.TrimSpace(branch.Key)
		if selectionKey == "" {
			selectionKey = strings.TrimSpace(branch.FullRef)
		}
		if selectionKey == "" {
			selectionKey = strings.TrimSpace(branch.QualifiedName)
		}
		if selectionKey == "" {
			selectionKey = strings.TrimSpace(branch.Name)
		}
		if _, exists := seen[selectionKey]; exists {
			continue
		}
		seen[selectionKey] = struct{}{}
		candidates = append(candidates, branch)
	}

	result.BranchCandidates = len(candidates)
	return result, candidates, nil
}

func collectMappedJiraGroups(repo config.RepoConfig, branches []model.BranchInfo) []string {
	groups := make(map[string]struct{})
	for _, branch := range branches {
		match, ok, _ := repo.ExtractJiraMatchDetailed(branch.Name)
		if !ok {
			continue
		}
		group := strings.TrimSpace(match.Group)
		if group == "" {
			continue
		}
		groups[group] = struct{}{}
	}

	result := make([]string, 0, len(groups))
	for group := range groups {
		result = append(result, group)
	}
	slices.Sort(result)
	return result
}

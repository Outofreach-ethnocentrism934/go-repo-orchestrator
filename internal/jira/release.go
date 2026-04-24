package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

const jiraReleasePageSize = 100
const jiraReleaseMaxRetries = 2
const jiraReleaseMaxRetryWait = 3 * time.Second

var numericReleaseIDRegex = regexp.MustCompile(`^\d+$`)
var jiraReleaseSleepFn = time.Sleep

func (s *StatusService) ListReleasedFixVersions(_ context.Context, group string) ([]ReleaseVersion, error) {
	if s == nil {
		return nil, nil
	}

	group = strings.TrimSpace(group)
	if group == "" {
		return nil, nil
	}

	groupCfg, ok := s.groups[group]
	if !ok {
		return nil, nil
	}
	if strings.TrimSpace(groupCfg.baseURL) == "" {
		return nil, nil
	}

	headers := buildRequestHeaders(groupCfg.auth)
	startAt := 0
	versions := make([]ReleaseVersion, 0, jiraReleasePageSize)

	for {
		requestURL, err := buildReleasedVersionsURL(groupCfg.baseURL, group, startAt)
		if err != nil {
			return nil, err
		}

		response, _, requestErr := s.resolveReleaseRequestWithRetry(group, groupCfg.transport, requestURL, headers)
		if requestErr != nil {
			return nil, fmt.Errorf("получить released fixVersion: %w", requestErr)
		}

		if reason, auth := detectAuthReason(response.statusCode, response.location, response.finalURL, response.contentType, response.body); auth {
			return nil, fmt.Errorf("jira release auth: %s", reason)
		}

		if response.statusCode != http.StatusOK {
			return nil, fmt.Errorf("jira release вернул http %d", response.statusCode)
		}

		page, parseErr := parseReleasedVersionsPage(response.body)
		if parseErr != nil {
			return nil, parseErr
		}

		versions = append(versions, page.versions...)
		if page.last {
			break
		}
		if page.nextStartAt <= startAt {
			break
		}
		startAt = page.nextStartAt
	}

	slices.SortStableFunc(versions, func(a, b ReleaseVersion) int {
		if a.ReleaseDate != b.ReleaseDate {
			if a.ReleaseDate > b.ReleaseDate {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return versions, nil
}

func (s *StatusService) ListDoneIssueKeysByRelease(_ context.Context, group, releaseID string) ([]string, error) {
	if s == nil {
		return nil, nil
	}

	group = strings.TrimSpace(group)
	releaseID = strings.TrimSpace(releaseID)
	if group == "" || releaseID == "" {
		return nil, nil
	}

	groupCfg, ok := s.groups[group]
	if !ok {
		return nil, nil
	}
	if strings.TrimSpace(groupCfg.baseURL) == "" {
		return nil, nil
	}

	headers := buildRequestHeaders(groupCfg.auth)
	startAt := 0
	keysSet := make(map[string]struct{})

	for {
		requestURL, err := buildDoneIssuesByReleaseURL(groupCfg.baseURL, releaseID, startAt)
		if err != nil {
			return nil, err
		}

		response, _, requestErr := s.resolveReleaseRequestWithRetry(group, groupCfg.transport, requestURL, headers)
		if requestErr != nil {
			return nil, fmt.Errorf("получить Jira issue keys по релизу: %w", requestErr)
		}

		if reason, auth := detectAuthReason(response.statusCode, response.location, response.finalURL, response.contentType, response.body); auth {
			return nil, fmt.Errorf("jira release auth: %s", reason)
		}

		if response.statusCode != http.StatusOK {
			return nil, fmt.Errorf("jira release search вернул http %d", response.statusCode)
		}

		page, parseErr := parseIssueKeysPage(response.body)
		if parseErr != nil {
			return nil, parseErr
		}

		for _, key := range page.keys {
			keysSet[key] = struct{}{}
		}

		if page.received == 0 {
			break
		}
		if page.total > 0 && startAt+page.received >= page.total {
			break
		}
		startAt += page.received
	}

	keys := make([]string, 0, len(keysSet))
	for key := range keysSet {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	return keys, nil
}

func (s *StatusService) resolveReleaseRequestWithRetry(group string, transport groupTransport, requestURL string, headers map[string]string) (searchStatusResponse, bool, error) {
	for attempt := 0; attempt <= jiraReleaseMaxRetries; attempt++ {
		response, usedBrowserFallback, requestErr := s.resolveSearch(group, transport, requestURL, headers)
		if requestErr != nil {
			return searchStatusResponse{}, usedBrowserFallback, requestErr
		}

		if response.statusCode == http.StatusTooManyRequests {
			if attempt >= jiraReleaseMaxRetries {
				return response, usedBrowserFallback, fmt.Errorf("jira release rate limit: http 429")
			}

			wait := releaseRetryDelay(response.retryAfter, attempt)
			jiraReleaseSleepFn(wait)
			continue
		}

		if response.statusCode >= 500 && response.statusCode <= 599 {
			if attempt >= jiraReleaseMaxRetries {
				return response, usedBrowserFallback, nil
			}

			jiraReleaseSleepFn(releaseRetryDelay("", attempt))
			continue
		}

		return response, usedBrowserFallback, nil
	}

	return searchStatusResponse{}, false, fmt.Errorf("jira release retry exhausted")
}

func releaseRetryDelay(retryAfter string, attempt int) time.Duration {
	if ttl, ok := parseRetryAfter(retryAfter); ok {
		if ttl > jiraReleaseMaxRetryWait {
			return jiraReleaseMaxRetryWait
		}
		if ttl < 100*time.Millisecond {
			return 100 * time.Millisecond
		}
		return ttl
	}

	base := 200 * time.Millisecond
	delay := base << attempt
	if delay > jiraReleaseMaxRetryWait {
		return jiraReleaseMaxRetryWait
	}
	return delay
}

type releasedVersionsPage struct {
	versions    []ReleaseVersion
	nextStartAt int
	last        bool
}

func buildReleasedVersionsURL(jiraBaseURL, group string, startAt int) (string, error) {
	base := normalizeBaseURL(jiraBaseURL)
	group = strings.TrimSpace(group)
	if base == "" || group == "" {
		return "", fmt.Errorf("url для jira releases неполный")
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("разобрать базовый url jira: %w", err)
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/rest/api/3/project/" + url.PathEscape(group) + "/versions"
	query := parsed.Query()
	query.Set("status", "released")
	query.Set("maxResults", fmt.Sprintf("%d", jiraReleasePageSize))
	if startAt > 0 {
		query.Set("startAt", fmt.Sprintf("%d", startAt))
	}
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func buildDoneIssuesByReleaseURL(jiraBaseURL, releaseID string, startAt int) (string, error) {
	base := normalizeBaseURL(jiraBaseURL)
	releaseID = strings.TrimSpace(releaseID)
	if base == "" || releaseID == "" {
		return "", fmt.Errorf("url для jira release search неполный")
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("разобрать базовый url jira: %w", err)
	}

	releaseClause := releaseID
	if !numericReleaseIDRegex.MatchString(releaseID) {
		releaseClause = "\"" + strings.ReplaceAll(releaseID, "\"", "\\\"") + "\""
	}
	jql := "statusCategory = done AND fixVersion = " + releaseClause

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/rest/api/3/search"
	query := parsed.Query()
	query.Set("jql", jql)
	query.Set("fields", "key")
	query.Set("maxResults", fmt.Sprintf("%d", jiraSearchBatchSize))
	if startAt > 0 {
		query.Set("startAt", fmt.Sprintf("%d", startAt))
	}
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func parseReleasedVersionsPage(body []byte) (releasedVersionsPage, error) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return releasedVersionsPage{}, fmt.Errorf("jira release вернул пустой ответ")
	}

	if strings.HasPrefix(trimmed, "[") {
		var raw []struct {
			ID          any    `json:"id"`
			Name        string `json:"name"`
			Released    bool   `json:"released"`
			ReleaseDate string `json:"releaseDate"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return releasedVersionsPage{}, fmt.Errorf("декодировать jira release list: %w", err)
		}

		versions := make([]ReleaseVersion, 0, len(raw))
		for _, item := range raw {
			if !item.Released {
				continue
			}
			id := strings.TrimSpace(fmt.Sprintf("%v", item.ID))
			if id == "" {
				continue
			}
			versions = append(versions, ReleaseVersion{ID: id, Name: strings.TrimSpace(item.Name), ReleaseDate: strings.TrimSpace(item.ReleaseDate)})
		}

		return releasedVersionsPage{versions: versions, last: true}, nil
	}

	var payload struct {
		Values []struct {
			ID          any    `json:"id"`
			Name        string `json:"name"`
			Released    bool   `json:"released"`
			ReleaseDate string `json:"releaseDate"`
		} `json:"values"`
		StartAt    int  `json:"startAt"`
		MaxResults int  `json:"maxResults"`
		Total      int  `json:"total"`
		IsLast     bool `json:"isLast"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return releasedVersionsPage{}, fmt.Errorf("декодировать jira release list: %w", err)
	}

	versions := make([]ReleaseVersion, 0, len(payload.Values))
	for _, item := range payload.Values {
		if !item.Released {
			continue
		}
		id := strings.TrimSpace(fmt.Sprintf("%v", item.ID))
		if id == "" {
			continue
		}
		versions = append(versions, ReleaseVersion{ID: id, Name: strings.TrimSpace(item.Name), ReleaseDate: strings.TrimSpace(item.ReleaseDate)})
	}

	nextStartAt := payload.StartAt + payload.MaxResults
	if payload.MaxResults <= 0 {
		nextStartAt = payload.StartAt + len(payload.Values)
	}
	last := payload.IsLast || (payload.Total > 0 && nextStartAt >= payload.Total)

	return releasedVersionsPage{
		versions:    versions,
		nextStartAt: nextStartAt,
		last:        last,
	}, nil
}

type issueKeysPage struct {
	keys     []string
	total    int
	received int
}

func parseIssueKeysPage(body []byte) (issueKeysPage, error) {
	var payload struct {
		Total  int `json:"total"`
		Issues []struct {
			Key string `json:"key"`
		} `json:"issues"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return issueKeysPage{}, fmt.Errorf("декодировать jira release issue search: %w", err)
	}

	keys := make([]string, 0, len(payload.Issues))
	for _, issue := range payload.Issues {
		key := strings.ToUpper(strings.TrimSpace(issue.Key))
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}

	return issueKeysPage{
		keys:     keys,
		total:    payload.Total,
		received: len(payload.Issues),
	}, nil
}

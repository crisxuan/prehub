package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prehub/prehub/backend/internal/domain"
	"github.com/prehub/prehub/backend/internal/editorial"
)

const defaultBaseURL = "https://api.github.com"

type Client struct {
	baseURL    string
	token      string
	apiVersion string
	httpClient *http.Client
}

type RepositoryResponse struct {
	ID              int64      `json:"id"`
	NodeID          string     `json:"node_id"`
	Name            string     `json:"name"`
	FullName        string     `json:"full_name"`
	Owner           Owner      `json:"owner"`
	HTMLURL         string     `json:"html_url"`
	URL             string     `json:"url"`
	Description     string     `json:"description"`
	Homepage        string     `json:"homepage"`
	DefaultBranch   string     `json:"default_branch"`
	Language        string     `json:"language"`
	StargazersCount int        `json:"stargazers_count"`
	ForksCount      int        `json:"forks_count"`
	WatchersCount   int        `json:"watchers_count"`
	OpenIssuesCount int        `json:"open_issues_count"`
	License         *License   `json:"license"`
	Fork            bool       `json:"fork"`
	Archived        bool       `json:"archived"`
	Disabled        bool       `json:"disabled"`
	PushedAt        *time.Time `json:"pushed_at"`
	CreatedAt       *time.Time `json:"created_at"`
	UpdatedAt       *time.Time `json:"updated_at"`
	Topics          []string   `json:"topics"`
}

type Owner struct {
	Login string `json:"login"`
}

type License struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type SearchResponse struct {
	TotalCount        int                  `json:"total_count"`
	IncompleteResults bool                 `json:"incomplete_results"`
	Items             []RepositoryResponse `json:"items"`
}

type ReadmeResponse struct {
	SHA      string `json:"sha"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

type RateLimitSnapshot struct {
	Limit     string
	Remaining string
	Reset     string
	Resource  string
}

func New(token string, apiVersion string) *Client {
	if apiVersion == "" {
		apiVersion = "2026-03-10"
	}

	return &Client{
		baseURL:    defaultBaseURL,
		token:      token,
		apiVersion: apiVersion,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) SearchRepositories(ctx context.Context, query string, perPage int) ([]RepositoryResponse, RateLimitSnapshot, error) {
	return c.SearchRepositoriesWithSort(ctx, query, perPage, "updated", "desc")
}

func (c *Client) SearchRepositoriesWithSort(ctx context.Context, query string, perPage int, sort string, order string) ([]RepositoryResponse, RateLimitSnapshot, error) {
	if strings.TrimSpace(query) == "" {
		return nil, RateLimitSnapshot{}, errors.New("github search query is required")
	}
	if perPage <= 0 || perPage > 100 {
		perPage = 30
	}
	if sort == "" {
		sort = "updated"
	}
	if order == "" {
		order = "desc"
	}

	endpoint := "/search/repositories?q=" + url.QueryEscape(query) + "&sort=" + url.QueryEscape(sort) + "&order=" + url.QueryEscape(order) + "&per_page=" + fmt.Sprintf("%d", perPage)
	var payload SearchResponse
	rate, err := c.get(ctx, endpoint, &payload)
	if err != nil {
		return nil, rate, err
	}
	return payload.Items, rate, nil
}

func (c *Client) GetRepository(ctx context.Context, owner string, repo string) (RepositoryResponse, RateLimitSnapshot, error) {
	var payload RepositoryResponse
	rate, err := c.get(ctx, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo), &payload)
	return payload, rate, err
}

func (c *Client) GetTopics(ctx context.Context, owner string, repo string) ([]string, RateLimitSnapshot, error) {
	var payload struct {
		Names []string `json:"names"`
	}
	rate, err := c.get(ctx, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/topics", &payload)
	return payload.Names, rate, err
}

func (c *Client) GetLanguages(ctx context.Context, owner string, repo string) (map[string]int64, RateLimitSnapshot, error) {
	payload := map[string]int64{}
	rate, err := c.get(ctx, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/languages", &payload)
	return payload, rate, err
}

func (c *Client) GetReadme(ctx context.Context, owner string, repo string) (string, string, RateLimitSnapshot, error) {
	var payload ReadmeResponse
	rate, err := c.get(ctx, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)+"/readme", &payload)
	if err != nil {
		return "", "", rate, err
	}
	if payload.Encoding != "base64" {
		return payload.SHA, payload.Content, rate, nil
	}
	content, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
	if err != nil {
		return payload.SHA, "", rate, err
	}
	return payload.SHA, string(content), rate, nil
}

func (c *Client) get(ctx context.Context, endpoint string, target any) (RateLimitSnapshot, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return RateLimitSnapshot{}, err
	}

	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", c.apiVersion)
	request.Header.Set("User-Agent", "PreHub/0.1")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return RateLimitSnapshot{}, err
	}
	defer response.Body.Close()

	rate := RateLimitSnapshot{
		Limit:     response.Header.Get("x-ratelimit-limit"),
		Remaining: response.Header.Get("x-ratelimit-remaining"),
		Reset:     response.Header.Get("x-ratelimit-reset"),
		Resource:  response.Header.Get("x-ratelimit-resource"),
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return rate, fmt.Errorf("github api %s returned %d: %s", endpoint, response.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return rate, err
	}
	return rate, nil
}

func ToDomainRepository(response RepositoryResponse, topics []string, summary string) domain.Repository {
	license := "unknown"
	if response.License != nil && response.License.Key != "" {
		license = response.License.Key
	}

	pushedAt := ""
	if response.PushedAt != nil {
		pushedAt = response.PushedAt.UTC().Format(time.RFC3339)
	}

	if len(topics) == 0 {
		topics = response.Topics
	}
	if summary == "" {
		summary = response.Description
	}

	repo := domain.Repository{
		FullName:    response.FullName,
		Owner:       response.Owner.Login,
		Name:        response.Name,
		HTMLURL:     response.HTMLURL,
		Description: response.Description,
		Language:    response.Language,
		Stars:       response.StargazersCount,
		Forks:       response.ForksCount,
		License:     license,
		PushedAt:    pushedAt,
		Topics:      topics,
		Summary:     summary,
	}
	return editorial.WriteRepositoryNarrative(repo)
}

func ParseRepositoryURL(rawURL string) (string, string, error) {
	value := strings.TrimSpace(rawURL)
	value = strings.TrimSuffix(value, ".git")
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "git@")
	value = strings.TrimPrefix(value, "github.com:")
	value = strings.TrimPrefix(value, "github.com/")

	parts := strings.Split(value, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("expected github.com/{owner}/{repo}")
	}
	return parts[0], parts[1], nil
}

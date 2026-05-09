package clickhouse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/prehub/prehub/backend/internal/domain"
)

const defaultEndpoint = "https://sql-clickhouse.clickhouse.com/"

type Client struct {
	endpoint   string
	user       string
	password   string
	httpClient *http.Client
}

type TrendFetchOptions struct {
	Window        string
	WindowStarted time.Time
	WindowEnded   time.Time
	BatchSize     int
}

func New(endpoint string, user string, password string) *Client {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = defaultEndpoint
	}
	if strings.TrimSpace(user) == "" {
		user = "demo"
	}
	return &Client{
		endpoint:   endpoint,
		user:       user,
		password:   password,
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *Client) FetchRepositoryTrends(ctx context.Context, repositories []string, options TrendFetchOptions) (map[string]domain.ExternalRepositoryTrend, error) {
	repositories = uniqueRepositoryNames(repositories)
	if len(repositories) == 0 {
		return map[string]domain.ExternalRepositoryTrend{}, nil
	}
	if options.BatchSize <= 0 || options.BatchSize > 500 {
		options.BatchSize = 250
	}
	options.WindowStarted = options.WindowStarted.UTC()
	options.WindowEnded = options.WindowEnded.UTC()
	if !options.WindowEnded.After(options.WindowStarted) {
		return nil, fmt.Errorf("window end must be after window start")
	}

	trends := map[string]domain.ExternalRepositoryTrend{}
	for start := 0; start < len(repositories); start += options.BatchSize {
		end := start + options.BatchSize
		if end > len(repositories) {
			end = len(repositories)
		}
		batch, err := c.fetchBatch(ctx, repositories[start:end], options)
		if err != nil {
			return trends, err
		}
		for fullName, trend := range batch {
			trends[strings.ToLower(fullName)] = trend
		}
	}
	return trends, nil
}

func (c *Client) fetchBatch(ctx context.Context, repositories []string, options TrendFetchOptions) (map[string]domain.ExternalRepositoryTrend, error) {
	query := buildTrendQuery(repositories, options)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewBufferString(query))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "text/plain; charset=utf-8")
	q := request.URL.Query()
	q.Set("user", c.user)
	if c.password != "" {
		q.Set("password", c.password)
	}
	request.URL.RawQuery = q.Encode()

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, fmt.Errorf("clickhouse returned %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Data []trendRow `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}

	trends := map[string]domain.ExternalRepositoryTrend{}
	for _, row := range payload.Data {
		fullName := strings.TrimSpace(row.RepoName)
		if fullName == "" {
			continue
		}
		key := strings.ToLower(fullName)
		trend := trends[key]
		if trend.RepositoryFullName == "" {
			trend.RepositoryFullName = fullName
			trend.Window = options.Window
			trend.WindowStartedAt = options.WindowStarted
			trend.WindowEndedAt = options.WindowEnded
		}
		bucketStarted, err := parseClickHouseTime(row.BucketStartedAt)
		if err != nil {
			return nil, err
		}
		starDelta, err := parseInt(row.StarDelta)
		if err != nil {
			return nil, err
		}
		activityEvents, err := parseInt(row.ActivityEvents)
		if err != nil {
			return nil, err
		}
		bucketEnded := bucketStarted.Add(bucketDuration(options.Window))
		if bucketEnded.After(options.WindowEnded) {
			bucketEnded = options.WindowEnded
		}
		trend.StarDelta += starDelta
		trend.ActivityEvents += activityEvents
		trend.Buckets = append(trend.Buckets, domain.ExternalTrendBucket{
			BucketStartedAt: bucketStarted,
			BucketEndedAt:   bucketEnded,
			StarDelta:       starDelta,
			ActivityEvents:  activityEvents,
		})
		trends[key] = trend
	}

	for key, trend := range trends {
		sort.Slice(trend.Buckets, func(i int, j int) bool {
			return trend.Buckets[i].BucketStartedAt.Before(trend.Buckets[j].BucketStartedAt)
		})
		trends[key] = trend
	}
	return trends, nil
}

type trendRow struct {
	RepoName        string `json:"repo_name"`
	BucketStartedAt string `json:"bucket_started_at"`
	StarDelta       any    `json:"star_delta"`
	ActivityEvents  any    `json:"activity_events"`
}

func buildTrendQuery(repositories []string, options TrendFetchOptions) string {
	values := make([]string, 0, len(repositories))
	for _, repo := range repositories {
		values = append(values, quoteClickHouseString(repo))
	}
	bucket := "INTERVAL 1 DAY"
	if options.Window == "1h" {
		bucket = "INTERVAL 5 MINUTE"
	} else if options.Window == "24h" {
		bucket = "INTERVAL 1 HOUR"
	}
	return fmt.Sprintf(`
SELECT
	repo_name,
	toStartOfInterval(created_at, %s) AS bucket_started_at,
	count() AS star_delta,
	count() AS activity_events
FROM github.events
WHERE repo_name IN (%s)
	AND created_at >= toDateTime('%s')
	AND created_at < toDateTime('%s')
	AND event_type = 'WatchEvent'
	AND action = 'started'
GROUP BY repo_name, bucket_started_at
ORDER BY repo_name, bucket_started_at
FORMAT JSON
`, bucket, strings.Join(values, ","), formatClickHouseTime(options.WindowStarted), formatClickHouseTime(options.WindowEnded))
}

func bucketDuration(window string) time.Duration {
	if window == "1h" {
		return 5 * time.Minute
	}
	if window == "24h" {
		return time.Hour
	}
	return 24 * time.Hour
}

func parseClickHouseTime(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC)
	if err == nil {
		return parsed.UTC(), nil
	}
	return time.Parse(time.RFC3339, value)
}

func formatClickHouseTime(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05")
}

func quoteClickHouseString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
}

func parseInt(value any) (int, error) {
	switch typed := value.(type) {
	case float64:
		return int(typed), nil
	case string:
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, err
		}
		return int(parsed), nil
	default:
		return 0, fmt.Errorf("unsupported integer value %T", value)
	}
}

func uniqueRepositoryNames(repositories []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, repo := range repositories {
		repo = strings.TrimSpace(repo)
		key := strings.ToLower(repo)
		if repo == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, repo)
	}
	sort.Strings(result)
	return result
}

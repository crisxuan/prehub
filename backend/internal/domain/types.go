package domain

import "time"

type Repository struct {
	FullName    string   `json:"fullName"`
	Owner       string   `json:"owner"`
	Name        string   `json:"name"`
	HTMLURL     string   `json:"htmlUrl"`
	AvatarURL   string   `json:"avatarUrl"`
	Description string   `json:"description"`
	Language    string   `json:"language"`
	Stars       int      `json:"stars"`
	Forks       int      `json:"forks"`
	License     string   `json:"license"`
	PushedAt    string   `json:"pushedAt"`
	Topics      []string `json:"topics"`
	Reason      string   `json:"reason"`
	Caveat      string   `json:"caveat"`
	Summary     string   `json:"summary"`
}

type ScoreBreakdown struct {
	Quality       int `json:"quality"`
	Popularity    int `json:"popularity"`
	Freshness     int `json:"freshness"`
	Momentum      int `json:"momentum"`
	Documentation int `json:"documentation"`
	Maintenance   int `json:"maintenance"`
	Community     int `json:"community"`
	License       int `json:"license"`
	Novelty       int `json:"novelty"`
}

type DailyPick struct {
	Date         string       `json:"date"`
	Category     string       `json:"category"`
	Theme        string       `json:"theme"`
	Primary      Repository   `json:"primary"`
	Alternatives []Repository `json:"alternatives"`
}

type DailyPickHistory struct {
	FromDate string      `json:"fromDate"`
	ToDate   string      `json:"toDate"`
	Days     int         `json:"days"`
	Category string      `json:"category"`
	Picks    []DailyPick `json:"picks"`
}

type SearchResponse struct {
	Query   string       `json:"query"`
	Intent  []string     `json:"intent"`
	Results []Repository `json:"results"`
}

type Candidate struct {
	ID           string          `json:"id"`
	Repository   Repository      `json:"repository"`
	Status       string          `json:"status"`
	QualityScore int             `json:"qualityScore"`
	Score        *ScoreBreakdown `json:"score,omitempty"`
	Source       string          `json:"source,omitempty"`
}

type AdminOverview struct {
	CandidateCount      int    `json:"candidateCount"`
	PendingReviewCount  int    `json:"pendingReviewCount"`
	ScheduledPickCount  int    `json:"scheduledPickCount"`
	LastRateLimitStatus string `json:"lastRateLimitStatus"`
}

type SubmitRepositoryInput struct {
	URL      string `json:"url"`
	Source   string `json:"source"`
	Priority string `json:"priority"`
}

type SubmitRepositoryResponse struct {
	Status    string     `json:"status"`
	Candidate *Candidate `json:"candidate,omitempty"`
	Message   string     `json:"message,omitempty"`
}

type PublishCandidateInput struct {
	Date     string `json:"date"`
	Theme    string `json:"theme"`
	Category string `json:"category"`
}

type RadarAPIHealth struct {
	Status    string `json:"status"`
	Remaining string `json:"remaining"`
	ResetAt   string `json:"resetAt"`
}

type RadarDataCoverage struct {
	Complete        bool   `json:"complete"`
	ObservedSince   string `json:"observedSince"`
	ObservedUntil   string `json:"observedUntil"`
	WindowStartedAt string `json:"windowStartedAt"`
}

type RadarOverview struct {
	Category       string            `json:"category"`
	Window         string            `json:"window"`
	MonitoredCount int               `json:"monitoredCount"`
	StarDelta      int               `json:"starDelta"`
	CandidateCount int               `json:"candidateCount"`
	APIHealth      RadarAPIHealth    `json:"apiHealth"`
	DataCoverage   RadarDataCoverage `json:"dataCoverage"`
	TopTrending    []RadarTrendItem  `json:"topTrending"`
	TopPotential   []RadarTrendItem  `json:"topPotential"`
	RecentEvents   []RadarEvent      `json:"recentEvents"`
}

type RadarTrendItem struct {
	Repository        Repository        `json:"repository"`
	Window            string            `json:"window"`
	StarDelta         int               `json:"starDelta"`
	ForkDelta         int               `json:"forkDelta"`
	IssueDelta        int               `json:"issueDelta"`
	ActivityEvents    int               `json:"activityEvents"`
	VelocityScore     float64           `json:"velocityScore"`
	AccelerationScore float64           `json:"accelerationScore"`
	TrendScore        float64           `json:"trendScore"`
	Explanation       string            `json:"explanation"`
	DataCoverage      RadarDataCoverage `json:"dataCoverage"`
}

type RadarEvent struct {
	RepositoryFullName string `json:"repositoryFullName"`
	EventType          string `json:"eventType"`
	ActorLogin         string `json:"actorLogin"`
	ActorAvatarURL     string `json:"actorAvatarUrl"`
	OccurredAt         string `json:"occurredAt"`
}

type RepositoryStarEvent struct {
	GitHubUserID int64
	Login        string
	AvatarURL    string
	StarredAt    time.Time
}

type RadarMetricPoint struct {
	CapturedAt string `json:"capturedAt"`
	Stars      int    `json:"stars"`
	Forks      int    `json:"forks"`
	OpenIssues int    `json:"openIssues"`
}

type RadarMetricSummary struct {
	StarDelta      int `json:"starDelta"`
	ForkDelta      int `json:"forkDelta"`
	ActivityEvents int `json:"activityEvents"`
}

type RadarMetricsResponse struct {
	Repository   Repository         `json:"repository"`
	Window       string             `json:"window"`
	Points       []RadarMetricPoint `json:"points"`
	Summary      RadarMetricSummary `json:"summary"`
	DataCoverage RadarDataCoverage  `json:"dataCoverage"`
}

type AddWatchlistInput struct {
	URL      string `json:"url"`
	Category string `json:"category"`
	Tier     string `json:"tier"`
}

type AddWatchlistResponse struct {
	Status     string     `json:"status"`
	Repository Repository `json:"repository"`
	Category   string     `json:"category"`
	Tier       string     `json:"tier"`
	RateLimit  any        `json:"rateLimit,omitempty"`
}

type MonitoredRepository struct {
	Repository Repository `json:"repository"`
	Category   string     `json:"category"`
	Tier       string     `json:"tier"`
}

type MonitoredRepositoryRef struct {
	RepositoryID string `json:"repositoryId"`
	FullName     string `json:"fullName"`
	Category     string `json:"category"`
}

type ExternalTrendBucket struct {
	BucketStartedAt time.Time `json:"bucketStartedAt"`
	BucketEndedAt   time.Time `json:"bucketEndedAt"`
	StarDelta       int       `json:"starDelta"`
	ActivityEvents  int       `json:"activityEvents"`
}

type ExternalRepositoryTrend struct {
	RepositoryFullName string                `json:"repositoryFullName"`
	Window             string                `json:"window"`
	WindowStartedAt    time.Time             `json:"windowStartedAt"`
	WindowEndedAt      time.Time             `json:"windowEndedAt"`
	StarDelta          int                   `json:"starDelta"`
	ActivityEvents     int                   `json:"activityEvents"`
	Buckets            []ExternalTrendBucket `json:"buckets"`
}

type RadarBackfillInput struct {
	Category  string   `json:"category"`
	Windows   []string `json:"windows"`
	Limit     int      `json:"limit"`
	BatchSize int      `json:"batchSize"`
}

type RadarBackfillWindowResult struct {
	Window          string `json:"window"`
	RepositoryCount int    `json:"repositoryCount"`
	MatchedCount    int    `json:"matchedCount"`
	StarDelta       int    `json:"starDelta"`
	ActivityEvents  int    `json:"activityEvents"`
	WindowStartedAt string `json:"windowStartedAt"`
	WindowEndedAt   string `json:"windowEndedAt"`
}

type RadarBackfillResponse struct {
	Status   string                      `json:"status"`
	Source   string                      `json:"source"`
	Category string                      `json:"category"`
	Results  []RadarBackfillWindowResult `json:"results"`
}

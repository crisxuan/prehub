package domain

type Repository struct {
	FullName    string   `json:"fullName"`
	Owner       string   `json:"owner"`
	Name        string   `json:"name"`
	HTMLURL     string   `json:"htmlUrl"`
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

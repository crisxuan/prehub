package sample

import (
	"strings"
	"time"

	"github.com/prehub/prehub/backend/internal/domain"
)

var Repositories = []domain.Repository{
	{
		FullName:    "vercel/next.js",
		Owner:       "vercel",
		Name:        "next.js",
		HTMLURL:     "https://github.com/vercel/next.js",
		Description: "The React framework for production web applications.",
		Language:    "TypeScript",
		Stars:       133000,
		Forks:       28000,
		License:     "MIT",
		PushedAt:    "2026-05-04T09:30:00Z",
		Topics:      []string{"react", "nextjs", "framework", "typescript"},
		Reason:      "适合构建需要 SEO、服务端渲染和快速迭代的现代 Web 产品。",
		Caveat:      "框架能力很完整，简单静态站点可能会觉得偏重。",
		Summary:     "Next.js 提供 App Router、Server Components、Route Handlers 和完整的 React 生产应用工具链。",
	},
	{
		FullName:    "langchain-ai/langchain",
		Owner:       "langchain-ai",
		Name:        "langchain",
		HTMLURL:     "https://github.com/langchain-ai/langchain",
		Description: "Build context-aware reasoning applications.",
		Language:    "Python",
		Stars:       118000,
		Forks:       19000,
		License:     "MIT",
		PushedAt:    "2026-05-03T16:00:00Z",
		Topics:      []string{"ai", "llm", "agents", "python"},
		Reason:      "适合快速搭建 LLM 应用、工具调用和 agent workflow 原型。",
		Caveat:      "生态变化快，生产使用前需要锁定版本并评估抽象成本。",
		Summary:     "LangChain 是 LLM 应用开发框架，覆盖模型调用、检索、工具、agent 和工作流编排。",
	},
	{
		FullName:    "charmbracelet/bubbletea",
		Owner:       "charmbracelet",
		Name:        "bubbletea",
		HTMLURL:     "https://github.com/charmbracelet/bubbletea",
		Description: "A powerful little TUI framework.",
		Language:    "Go",
		Stars:       32000,
		Forks:       900,
		License:     "MIT",
		PushedAt:    "2026-04-29T12:00:00Z",
		Topics:      []string{"go", "cli", "tui", "terminal"},
		Reason:      "适合用 Go 构建终端工具、交互式 CLI 和开发者效率产品。",
		Caveat:      "TUI 交互模型需要适应 Elm-style update loop。",
		Summary:     "Bubble Tea 是 Go 生态里成熟的 TUI 框架，常用于构建漂亮的终端应用。",
	},
	{
		FullName:    "supabase/supabase",
		Owner:       "supabase",
		Name:        "supabase",
		HTMLURL:     "https://github.com/supabase/supabase",
		Description: "The open source Firebase alternative.",
		Language:    "TypeScript",
		Stars:       89000,
		Forks:       9400,
		License:     "Apache-2.0",
		PushedAt:    "2026-05-01T08:15:00Z",
		Topics:      []string{"database", "postgres", "self-hosted", "typescript"},
		Reason:      "适合想要 Postgres、Auth、Storage 和实时能力的一体化开源后端。",
		Caveat:      "完整自托管需要理解多个服务组件，运维复杂度不低。",
		Summary:     "Supabase 以 Postgres 为核心，提供数据库、认证、存储、边缘函数和实时订阅能力。",
	},
}

func TodayPick() domain.DailyPick {
	return domain.DailyPick{
		Date:         "2026-05-07",
		Category:     domain.DefaultCategory,
		Theme:        "现代 Web 与开发者工具",
		Primary:      Repositories[0],
		Alternatives: Repositories[1:],
	}
}

func RecentDailyPicks(days int, now time.Time) domain.DailyPickHistory {
	if days <= 0 {
		days = 7
	}
	to := now.UTC()
	from := to.AddDate(0, 0, -(days - 1))
	return domain.DailyPickHistory{
		FromDate: from.Format("2006-01-02"),
		ToDate:   to.Format("2006-01-02"),
		Days:     days,
		Category: domain.DefaultCategory,
		Picks:    []domain.DailyPick{TodayPick()},
	}
}

func Search(query string) domain.SearchResponse {
	normalized := strings.ToLower(strings.TrimSpace(query))
	results := make([]domain.Repository, 0, len(Repositories))
	for _, repo := range Repositories {
		if normalized == "" || matches(repo, normalized) {
			results = append(results, repo)
		}
	}

	intent := Intent(query)

	return domain.SearchResponse{
		Query:   query,
		Intent:  intent,
		Results: results,
	}
}

func Intent(query string) []string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	intent := []string{"repository-discovery"}
	if strings.Contains(normalized, "ai") || strings.Contains(normalized, "llm") || strings.Contains(normalized, "agent") {
		intent = append(intent, "ai")
	}
	if strings.Contains(normalized, "go") || strings.Contains(normalized, "cli") {
		intent = append(intent, "developer-tools")
	}
	if strings.Contains(normalized, "next") || strings.Contains(normalized, "react") {
		intent = append(intent, "web-framework")
	}
	return intent
}

func Find(owner string, repoName string) (domain.Repository, bool) {
	fullName := strings.ToLower(owner + "/" + repoName)
	for _, repo := range Repositories {
		if strings.ToLower(repo.FullName) == fullName {
			return repo, true
		}
	}
	return domain.Repository{}, false
}

func Candidates() []domain.Candidate {
	return []domain.Candidate{
		{ID: "cand_nextjs", Repository: Repositories[0], Status: "pending_review", QualityScore: 92},
		{ID: "cand_langchain", Repository: Repositories[1], Status: "scored", QualityScore: 87},
		{ID: "cand_bubbletea", Repository: Repositories[2], Status: "approved", QualityScore: 81},
	}
}

func matches(repo domain.Repository, query string) bool {
	if strings.Contains(strings.ToLower(repo.FullName), query) ||
		strings.Contains(strings.ToLower(repo.Description), query) ||
		strings.Contains(strings.ToLower(repo.Language), query) {
		return true
	}
	for _, topic := range repo.Topics {
		if strings.Contains(strings.ToLower(topic), query) {
			return true
		}
	}
	return false
}

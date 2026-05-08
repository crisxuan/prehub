package domain

import (
	"fmt"
	"strings"
	"time"
)

type RecommendationCategory struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
}

const (
	AllCategory     = "all"
	DefaultCategory = "ai"
)

var RecommendationCategories = []RecommendationCategory{
	{Slug: "ai", Label: "AI"},
	{Slug: "ai-image", Label: "AI 绘图/多模态"},
	{Slug: "ai-prompts", Label: "Prompt 技巧"},
	{Slug: "ai-skills", Label: "AI Skills/工作流"},
	{Slug: "web", Label: "Web 前端"},
	{Slug: "devtools", Label: "开发工具"},
	{Slug: "data", Label: "数据与数据库"},
	{Slug: "backend", Label: "后端基础设施"},
}

func NormalizeCategory(category string) string {
	value := strings.ToLower(strings.TrimSpace(category))
	if value == AllCategory {
		return AllCategory
	}
	for _, item := range RecommendationCategories {
		if value == item.Slug {
			return value
		}
	}
	return DefaultCategory
}

func CategoryLabel(category string) string {
	normalized := NormalizeCategory(category)
	if normalized == AllCategory {
		return "全部"
	}
	for _, item := range RecommendationCategories {
		if item.Slug == normalized {
			return item.Label
		}
	}
	return "AI"
}

func IsAllCategory(category string) bool {
	return NormalizeCategory(category) == AllCategory
}

func DefaultSearchQuery(category string) string {
	queries := DefaultSearchQueries(category)
	if len(queries) == 0 {
		return ""
	}
	return queries[0]
}

func DefaultSearchQueries(category string) []string {
	return DiscoverySearchQueries(category, time.Now().UTC())
}

func DiscoverySearchQueries(category string, now time.Time) []string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	pushedSince := now.AddDate(0, -6, 0).Format(time.DateOnly)
	createdSince := now.AddDate(0, -3, 0).Format(time.DateOnly)
	base := fmt.Sprintf("stars:10..20000 pushed:>%s archived:false fork:false", pushedSince)
	newer := fmt.Sprintf("stars:5..8000 created:>%s archived:false fork:false", createdSince)

	switch NormalizeCategory(category) {
	case "ai-image":
		return []string{
			`"gpt-image" in:name,description,readme ` + base,
			`"image generation" in:name,description,readme ` + base,
			`"comfyui workflow" in:name,description,readme ` + base,
			`"multimodal" "image" in:name,description,readme ` + newer,
		}
	case "ai-prompts":
		return []string{
			`"prompt engineering" in:name,description,readme ` + base,
			`"system prompt" in:name,description,readme ` + base,
			`"prompt template" in:name,description,readme ` + base,
			`"agent prompts" in:name,description,readme ` + newer,
		}
	case "ai-skills":
		return []string{
			`"agent skill" in:name,description,readme ` + base,
			`"codex skill" in:name,description,readme ` + base,
			`"mcp server" in:name,description,readme ` + base,
			`"workflow" "agent" in:name,description,readme ` + newer,
		}
	case "web":
		return []string{
			`topic:nextjs ` + base,
			`"react framework" in:name,description,readme ` + base,
			`"ui components" in:name,description,readme ` + newer,
		}
	case "devtools":
		return []string{
			`topic:developer-tools ` + base,
			`"cli" "developer" in:name,description,readme ` + newer,
			`"code agent" in:name,description,readme ` + base,
		}
	case "data":
		return []string{
			`topic:database ` + base,
			`"vector database" in:name,description,readme ` + base,
			`"data pipeline" in:name,description,readme ` + newer,
		}
	case "backend":
		return []string{
			`topic:backend ` + base,
			`"api gateway" in:name,description,readme ` + base,
			`"server framework" in:name,description,readme ` + newer,
		}
	case "ai":
		fallthrough
	default:
		return []string{
			"topic:ai " + base,
			"topic:llm " + base,
			`"ai agent" in:name,description,readme ` + base,
			`"rag" in:name,description,readme ` + base,
			`"model routing" in:name,description,readme ` + newer,
			`"llm eval" in:name,description,readme ` + newer,
			`"mcp" "agent" in:name,description,readme ` + newer,
		}
	}
}

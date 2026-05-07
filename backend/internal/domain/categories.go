package domain

import "strings"

type RecommendationCategory struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
}

const DefaultCategory = "ai"

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
	for _, item := range RecommendationCategories {
		if value == item.Slug {
			return value
		}
	}
	return DefaultCategory
}

func CategoryLabel(category string) string {
	normalized := NormalizeCategory(category)
	for _, item := range RecommendationCategories {
		if item.Slug == normalized {
			return item.Label
		}
	}
	return "AI"
}

func DefaultSearchQuery(category string) string {
	queries := DefaultSearchQueries(category)
	if len(queries) == 0 {
		return ""
	}
	return queries[0]
}

func DefaultSearchQueries(category string) []string {
	switch NormalizeCategory(category) {
	case "ai-image":
		return []string{
			`"gpt-image" in:name,description,readme stars:10..12000 pushed:>2026-02-01 archived:false fork:false`,
			`"image generation" in:name,description,readme stars:10..12000 pushed:>2026-02-01 archived:false fork:false`,
			`"comfyui workflow" in:name,description,readme stars:10..12000 pushed:>2026-02-01 archived:false fork:false`,
		}
	case "ai-prompts":
		return []string{
			`"prompt engineering" in:name,description,readme stars:10..12000 pushed:>2026-02-01 archived:false fork:false`,
			`"system prompt" in:name,description,readme stars:10..12000 pushed:>2026-02-01 archived:false fork:false`,
			`"prompt template" in:name,description,readme stars:10..12000 pushed:>2026-02-01 archived:false fork:false`,
		}
	case "ai-skills":
		return []string{
			`"agent skill" in:name,description,readme stars:10..12000 pushed:>2026-02-01 archived:false fork:false`,
			`"codex skill" in:name,description,readme stars:10..12000 pushed:>2026-02-01 archived:false fork:false`,
			`"mcp server" in:name,description,readme stars:10..12000 pushed:>2026-02-01 archived:false fork:false`,
		}
	case "ai":
		fallthrough
	default:
		return []string{"topic:ai stars:100..12000 pushed:>2026-02-01 archived:false fork:false"}
	}
}

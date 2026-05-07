package domain

import "strings"

type RecommendationCategory struct {
	Slug  string `json:"slug"`
	Label string `json:"label"`
}

const DefaultCategory = "ai"

var RecommendationCategories = []RecommendationCategory{
	{Slug: "ai", Label: "AI"},
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

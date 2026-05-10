package main

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/prehub/prehub/backend/internal/domain"
)

func TestBuildArticleUsesSixNumberedSectionsAndClosingLine(t *testing.T) {
	now := time.Date(2026, 5, 10, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	article := buildArticle("2026-05-10", "all", []candidate{testCandidate("StarRocks/starrocks")}, now, nil)

	headings := regexp.MustCompile(`(?m)^## .+$`).FindAllString(article, -1)
	wantHeadings := []string{
		"## 1. 它到底是什么",
		"## 2. 它解决什么麻烦",
		"## 3. 核心看点",
		"## 4. 为什么值得看",
		"## 5. 怎么用起来",
		"## 6. 适合谁，以及先注意什么",
	}
	if len(headings) != len(wantHeadings) {
		t.Fatalf("heading count = %d, want %d\nheadings: %v", len(headings), len(wantHeadings), headings)
	}
	for index := range wantHeadings {
		if headings[index] != wantHeadings[index] {
			t.Fatalf("heading %d = %q, want %q", index, headings[index], wantHeadings[index])
		}
	}
	if strings.Contains(article, "顺手再看") || strings.Contains(article, "相关项目") {
		t.Fatalf("article contains related-project language")
	}
	if got := lastNonEmptyLine(article); got != "今天就先聊到这里。" {
		t.Fatalf("last line = %q, want closing line", got)
	}
}

func TestArticleThemesUseRequiredTitles(t *testing.T) {
	now := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	required := []string{"它到底是什么", "它解决什么麻烦", "核心看点", "为什么值得看", "怎么用起来", "适合谁，以及先注意什么"}
	cases := []candidate{
		testCandidate("StarRocks/starrocks"),
		testCandidate("woodpecker-ci/woodpecker"),
		testCandidate("example/toolbox"),
	}

	for _, item := range cases {
		themes := articleThemes(item, now)
		if len(themes) != len(required) {
			t.Fatalf("%s theme count = %d, want %d", item.repo.FullName, len(themes), len(required))
		}
		for index := range required {
			if themes[index].title != required[index] {
				t.Fatalf("%s theme %d = %q, want %q", item.repo.FullName, index, themes[index].title, required[index])
			}
		}
	}
}

func testCandidate(fullName string) candidate {
	owner, name, _ := strings.Cut(fullName, "/")
	return candidate{
		repo: domain.Repository{
			FullName:    fullName,
			Owner:       owner,
			Name:        name,
			HTMLURL:     "https://github.com/" + fullName,
			AvatarURL:   "https://github.com/" + owner + ".png?size=240",
			Description: "A practical developer tool for analytics and automation",
			Language:    "Go",
			Stars:       11656,
			Forks:       2408,
			License:     "apache-2.0",
			PushedAt:    "2026-05-10T00:00:00Z",
			Topics:      []string{"developer-tools", "analytics", "automation"},
			Summary:     "A practical developer tool for analytics and automation",
		},
	}
}

func lastNonEmptyLine(value string) string {
	lines := strings.Split(value, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if line != "" {
			return line
		}
	}
	return ""
}

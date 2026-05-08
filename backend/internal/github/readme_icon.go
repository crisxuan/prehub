package github

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

var (
	markdownImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	htmlImagePattern     = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	htmlAttributePattern = regexp.MustCompile(`(?is)([a-z0-9_-]+)\s*=\s*["']([^"']+)["']`)
)

type readmeImageCandidate struct {
	alt   string
	src   string
	order int
}

// ResolveReadmeIconURL finds a likely project logo/icon from README images and
// converts repository-relative paths into raw GitHub URLs.
func ResolveReadmeIconURL(readme string, owner string, repo string, defaultBranch string) string {
	readme = strings.TrimSpace(readme)
	if readme == "" {
		return ""
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	candidates := readmeImageCandidates(readme)
	bestScore := 0
	bestOrder := len(candidates) + 1
	bestURL := ""
	for _, candidate := range candidates {
		resolved := resolveRepositoryAssetURL(candidate.src, owner, repo, defaultBranch)
		if resolved == "" {
			continue
		}
		score := readmeIconScore(candidate.alt, resolved, candidate.order)
		if score <= 0 {
			continue
		}
		if score > bestScore || score == bestScore && candidate.order < bestOrder {
			bestScore = score
			bestOrder = candidate.order
			bestURL = resolved
		}
	}
	return bestURL
}

func readmeImageCandidates(readme string) []readmeImageCandidate {
	candidates := []readmeImageCandidate{}
	for _, match := range markdownImagePattern.FindAllStringSubmatch(readme, -1) {
		if len(match) < 3 {
			continue
		}
		candidates = append(candidates, readmeImageCandidate{
			alt:   strings.TrimSpace(match[1]),
			src:   strings.Trim(strings.TrimSpace(match[2]), `"'`),
			order: len(candidates),
		})
	}

	for _, tag := range htmlImagePattern.FindAllString(readme, -1) {
		attrs := map[string]string{}
		for _, match := range htmlAttributePattern.FindAllStringSubmatch(tag, -1) {
			if len(match) < 3 {
				continue
			}
			attrs[strings.ToLower(match[1])] = strings.TrimSpace(match[2])
		}
		if attrs["src"] == "" {
			continue
		}
		candidates = append(candidates, readmeImageCandidate{
			alt:   attrs["alt"],
			src:   attrs["src"],
			order: len(candidates),
		})
	}
	return candidates
}

func readmeIconScore(alt string, src string, order int) int {
	value := strings.ToLower(alt + " " + src)
	if isBadgeLikeImage(value) || isNonIconImage(value) {
		return 0
	}

	if !containsAny(value, "logo", "icon", "brand", "mark", "avatar") {
		return 0
	}
	score := 80
	if containsAny(value, "/assets/", "/asset/", "/docs/", "/doc/", "/media/", "/public/", "/static/", "/.github/") {
		score += 15
	}
	if containsAny(value, ".svg", ".png", ".webp", ".jpg", ".jpeg", ".gif") {
		score += 10
	}
	if order < 3 {
		score += 8
	} else if order < 8 {
		score += 3
	}
	return score
}

func isBadgeLikeImage(value string) bool {
	return containsAny(
		value,
		"badge",
		"shields.io",
		"badgen.net",
		"img.shields",
		"github/actions",
		"actions/workflows",
		"travis-ci",
		"circleci",
		"appveyor",
		"codecov",
		"coveralls",
		"snyk.io/test",
		"renovatebot",
		"version",
		"downloads",
		"license",
		"coverage",
		"build-status",
	)
}

func isNonIconImage(value string) bool {
	return containsAny(value, "screenshot", "preview", "demo", "banner", "hero", "cover")
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func resolveRepositoryAssetURL(src string, owner string, repo string, defaultBranch string) string {
	src = strings.TrimSpace(strings.Trim(src, `"'`))
	if src == "" || strings.HasPrefix(strings.ToLower(src), "data:") {
		return ""
	}
	if strings.HasPrefix(src, "//") {
		return "https:" + src
	}
	parsed, err := url.Parse(src)
	if err == nil && parsed.Scheme != "" {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return ""
		}
		return normalizeAbsoluteImageURL(parsed)
	}
	if strings.HasPrefix(src, "#") {
		return ""
	}

	assetPath := src
	if err == nil && parsed.Path != "" {
		assetPath = parsed.Path
	}
	cleanPath := strings.TrimPrefix(assetPath, "./")
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	if cleanPath == "" || strings.HasPrefix(cleanPath, "../") {
		return ""
	}

	escapedPath := strings.TrimPrefix(path.Clean(cleanPath), "/")
	resolved := "https://raw.githubusercontent.com/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/" + url.PathEscape(defaultBranch) + "/" + escapePathSegments(escapedPath)
	if err == nil && parsed.RawQuery != "" {
		resolved += "?" + parsed.RawQuery
	}
	return resolved
}

func normalizeAbsoluteImageURL(parsed *url.URL) string {
	if strings.EqualFold(parsed.Host, "github.com") {
		segments := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
		if len(segments) >= 5 && segments[2] == "blob" {
			return "https://raw.githubusercontent.com/" + url.PathEscape(segments[0]) + "/" + url.PathEscape(segments[1]) + "/" + url.PathEscape(segments[3]) + "/" + escapePathSegments(strings.Join(segments[4:], "/"))
		}
	}
	return parsed.String()
}

func escapePathSegments(value string) string {
	segments := strings.Split(value, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

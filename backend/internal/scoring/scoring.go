package scoring

import (
	"math"
	"strings"
	"time"

	"github.com/prehub/prehub/backend/internal/domain"
)

const MinimumCandidateQuality = 50

func ScoreRepository(repo domain.Repository, now time.Time) domain.ScoreBreakdown {
	popularity := adoptionScore(repo.Stars)
	freshness := freshnessScore(repo.PushedAt, now)
	documentation := documentationScore(repo)
	maintenance := maintenanceScore(repo, freshness)
	community := clamp(int(math.Log10(float64(max(repo.Forks, 1)))*28), 0, 100)
	license := licenseScore(repo.License)
	novelty := noveltyScore(repo.Stars, freshness)
	momentum := clamp((freshness+novelty+community/2)/2, 0, 100)

	quality := int(
		0.14*float64(popularity) +
			0.22*float64(freshness) +
			0.16*float64(momentum) +
			0.16*float64(documentation) +
			0.10*float64(maintenance) +
			0.08*float64(community) +
			0.06*float64(license) +
			0.08*float64(novelty),
	)
	if riskPenalty(repo) > 0 {
		quality = min(quality, 35)
		novelty = min(novelty, 20)
	}
	if projectFitPenalty(repo) > 0 {
		quality = min(quality, 45)
		novelty = min(novelty, 25)
	}

	return domain.ScoreBreakdown{
		Quality:       clamp(quality, 0, 100),
		Popularity:    popularity,
		Freshness:     freshness,
		Momentum:      momentum,
		Documentation: documentation,
		Maintenance:   maintenance,
		Community:     community,
		License:       license,
		Novelty:       novelty,
	}
}

func IsCandidateQualityAcceptable(score domain.ScoreBreakdown) bool {
	return score.Quality >= MinimumCandidateQuality
}

func adoptionScore(stars int) int {
	switch {
	case stars < 50:
		return 25
	case stars < 100:
		return 38
	case stars < 300:
		return 62
	case stars < 1500:
		return 72
	case stars < 8000:
		return 95
	case stars < 12000:
		return 78
	case stars < 30000:
		return 55
	case stars < 50000:
		return 42
	default:
		return 20
	}
}

func freshnessScore(pushedAt string, now time.Time) int {
	if strings.TrimSpace(pushedAt) == "" {
		return 20
	}
	parsed, err := time.Parse(time.RFC3339, pushedAt)
	if err != nil {
		return 30
	}
	days := now.Sub(parsed).Hours() / 24
	switch {
	case days <= 14:
		return 100
	case days <= 45:
		return 88
	case days <= 90:
		return 72
	case days <= 180:
		return 55
	case days <= 365:
		return 35
	default:
		return 15
	}
}

func documentationScore(repo domain.Repository) int {
	score := 30
	if len(repo.Summary) > 80 {
		score += 25
	}
	if len(repo.Description) > 30 {
		score += 15
	}
	if len(repo.Topics) >= 3 {
		score += 15
	}
	if repo.Reason != "" && repo.Caveat != "" {
		score += 15
	}
	return clamp(score, 0, 100)
}

func maintenanceScore(repo domain.Repository, freshness int) int {
	score := freshness
	if repo.Stars > 1000 {
		score += 8
	}
	if repo.Forks > 100 {
		score += 5
	}
	return clamp(score, 0, 100)
}

func licenseScore(license string) int {
	switch strings.ToLower(strings.TrimSpace(license)) {
	case "mit", "apache-2.0", "bsd-2-clause", "bsd-3-clause", "isc":
		return 100
	case "mpl-2.0", "lgpl-3.0", "lgpl-2.1":
		return 72
	case "gpl-2.0", "gpl-3.0", "agpl-3.0":
		return 45
	case "", "unknown", "other":
		return 20
	default:
		return 55
	}
}

func noveltyScore(stars int, freshness int) int {
	switch {
	case stars >= 200 && stars < 8000 && freshness >= 70:
		return 96
	case stars >= 100 && stars < 12000 && freshness >= 70:
		return 86
	case stars >= 12000 && stars < 30000 && freshness >= 60:
		return 55
	case stars < 200 && freshness >= 80:
		return 70
	case stars > 30000:
		return 15
	default:
		return 55
	}
}

func riskPenalty(repo domain.Repository) int {
	haystack := strings.ToLower(strings.Join([]string{
		repo.FullName,
		repo.Description,
		repo.Summary,
		strings.Join(repo.Topics, " "),
	}, " "))

	hasCredentialTerm := containsAny(haystack, []string{
		"api key", "api keys", "api-key", "api-keys", "token leak", "credential",
	})
	hasFreeCredentialSignal := hasCredentialTerm && containsAny(haystack, []string{
		"free ", "free-", "no credit card", "copy, paste", "leaked",
	})
	if hasFreeCredentialSignal {
		return 70
	}
	if containsAny(haystack, []string{"crack", "bypass paywall", "stolen token"}) {
		return 70
	}
	return 0
}

func projectFitPenalty(repo domain.Repository) int {
	haystack := strings.ToLower(strings.Join([]string{
		repo.FullName,
		repo.Description,
		repo.Summary,
		strings.Join(repo.Topics, " "),
	}, " "))

	if strings.HasPrefix(strings.ToLower(repo.Name), "awesome") || containsAny(haystack, []string{"awesome-list", "curated list", "comprehensive list", "comprehensive collection"}) {
		return 50
	}
	if strings.Contains(haystack, "feedback for ") {
		return 50
	}
	return 0
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func clamp(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

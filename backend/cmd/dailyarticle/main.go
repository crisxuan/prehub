package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/prehub/prehub/backend/internal/domain"
	"github.com/prehub/prehub/backend/internal/editorial"
	gh "github.com/prehub/prehub/backend/internal/github"
	"github.com/prehub/prehub/backend/internal/scoring"
)

const (
	defaultCategory   = "all"
	defaultSinceDays  = 45
	defaultPickCount  = 5
	defaultRecentDays = 30
	defaultTimeZone   = "Asia/Shanghai"
	defaultAPIVersion = "2022-11-28"
)

type options struct {
	outputDir       string
	category        string
	date            string
	timeZone        string
	token           string
	apiVersion      string
	imageBedDir     string
	imageBedBaseURL string
	imageBedPath    string
	sinceDays       int
	pickCount       int
	recentRepoDays  int
	imageBedPush    bool
	force           bool
	dryRun          bool
}

type searchPlan struct {
	query string
	sort  string
	order string
}

type candidate struct {
	response gh.RepositoryResponse
	repo     domain.Repository
	score    domain.ScoreBreakdown
	source   string
	readme   string
	imageURL string
}

type recentRepoSet struct {
	fullNames map[string]bool
	safeKeys  map[string]bool
}

type articleTheme struct {
	title    string
	body     string
	imageURL string
	imageAlt string
}

type imageStore struct {
	assetDir        string
	relDir          string
	imageBedDir     string
	imageBedBaseURL string
	imageBedPath    string
	imageBedPrefix  string
	imageBedFiles   []string
	index           int
	imageBedPush    bool
	seen            map[string]string
	client          *http.Client
}

var (
	markdownImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	htmlImagePattern     = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	htmlAttributePattern = regexp.MustCompile(`(?is)([a-z0-9_-]+)\s*=\s*["']([^"']+)["']`)
	filenamePattern      = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)
)

func main() {
	log.SetFlags(0)

	opts := parseOptions()
	if err := run(context.Background(), opts); err != nil {
		log.Fatal(err)
	}
}

func parseOptions() options {
	localEnv := loadLocalEnv()
	home, _ := os.UserHomeDir()
	defaultOutputDir := filepath.Join(home, "Downloads", "note", "github")
	if value := envFrom("GITHUB_DAILY_OUTPUT_DIR", "", localEnv); value != "" {
		defaultOutputDir = value
	}
	defaultImageBedDir := filepath.Join(home, "Downloads", "picbed")

	opts := options{
		outputDir:       defaultOutputDir,
		category:        envFrom("GITHUB_DAILY_CATEGORY", defaultCategory, localEnv),
		date:            envFrom("GITHUB_DAILY_DATE", "", localEnv),
		timeZone:        envFrom("PREHUB_TIMEZONE", defaultTimeZone, localEnv),
		token:           envFrom("GITHUB_TOKEN", "", localEnv),
		apiVersion:      envFrom("GITHUB_API_VERSION", defaultAPIVersion, localEnv),
		imageBedDir:     envFrom("GITHUB_DAILY_IMAGE_BED_DIR", defaultImageBedDir, localEnv),
		imageBedBaseURL: envFrom("GITHUB_DAILY_IMAGE_BED_BASE_URL", "https://cdn.jsdelivr.net/gh/doggaifan/picbed", localEnv),
		imageBedPath:    envFrom("GITHUB_DAILY_IMAGE_BED_PATH", "", localEnv),
		sinceDays:       envIntFrom("GITHUB_DAILY_SINCE_DAYS", defaultSinceDays, localEnv),
		pickCount:       envIntFrom("GITHUB_DAILY_PICK_COUNT", defaultPickCount, localEnv),
		recentRepoDays:  envIntFrom("GITHUB_DAILY_RECENT_REPO_DAYS", defaultRecentDays, localEnv),
		imageBedPush:    envBoolFrom("GITHUB_DAILY_IMAGE_BED_PUSH", true, localEnv),
	}

	flag.StringVar(&opts.outputDir, "output-dir", opts.outputDir, "directory for generated markdown articles")
	flag.StringVar(&opts.category, "category", opts.category, "recommendation category: all, ai, devtools, web, data, backend")
	flag.StringVar(&opts.date, "date", opts.date, "article date in YYYY-MM-DD; defaults to today in the configured timezone")
	flag.StringVar(&opts.timeZone, "timezone", opts.timeZone, "IANA timezone for article dates")
	flag.StringVar(&opts.imageBedDir, "image-bed-dir", opts.imageBedDir, "local GitHub picbed repository directory; empty disables picbed uploads")
	flag.StringVar(&opts.imageBedBaseURL, "image-bed-base-url", opts.imageBedBaseURL, "public CDN base URL for the GitHub picbed repository")
	flag.StringVar(&opts.imageBedPath, "image-bed-path", opts.imageBedPath, "optional repository subdirectory for uploaded picbed images")
	flag.IntVar(&opts.sinceDays, "since-days", opts.sinceDays, "freshness window for GitHub search")
	flag.IntVar(&opts.pickCount, "pick-count", opts.pickCount, "number of projects to include")
	flag.IntVar(&opts.recentRepoDays, "recent-repo-days", opts.recentRepoDays, "number of recent article days to exclude from selection")
	flag.BoolVar(&opts.imageBedPush, "image-bed-push", opts.imageBedPush, "commit and push downloaded article images to the GitHub picbed repository")
	flag.BoolVar(&opts.force, "force", false, "overwrite today's article if it already exists")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "print the article instead of writing a file")
	flag.Parse()

	if opts.pickCount < 2 {
		opts.pickCount = 2
	}
	if opts.pickCount > 8 {
		opts.pickCount = 8
	}
	if opts.sinceDays < 7 {
		opts.sinceDays = 7
	}
	if opts.recentRepoDays < 1 {
		opts.recentRepoDays = 1
	}
	return opts
}

func run(ctx context.Context, opts options) error {
	location, err := time.LoadLocation(opts.timeZone)
	if err != nil {
		return fmt.Errorf("load timezone %q: %w", opts.timeZone, err)
	}

	now := time.Now().In(location)
	articleDate := now.Format("2006-01-02")
	if opts.date != "" {
		parsed, err := time.ParseInLocation("2006-01-02", opts.date, location)
		if err != nil {
			return fmt.Errorf("parse --date: %w", err)
		}
		now = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 12, 0, 0, 0, location)
		articleDate = parsed.Format("2006-01-02")
	}

	client := gh.New(opts.token, opts.apiVersion)
	candidates, err := collectCandidates(ctx, client, opts, now)
	if err != nil {
		return err
	}
	if len(candidates) < 1 {
		return fmt.Errorf("not enough GitHub candidates collected: %d", len(candidates))
	}

	selected := selectCandidates(candidates, opts.pickCount)
	outputPath := filepath.Join(opts.outputDir, articleDate+"-github-daily.md")

	if opts.dryRun {
		article := buildArticle(articleDate, opts.category, selected, now, nil)
		fmt.Print(article)
		return nil
	}

	if _, err := os.Stat(outputPath); err == nil && !opts.force {
		fmt.Printf("article already exists: %s\n", outputPath)
		fmt.Printf("primary repo: %s\n", selected[0].repo.FullName)
		return nil
	}
	if err := os.MkdirAll(opts.outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	images, err := newImageStore(opts, articleDate, selected[0].repo)
	if err != nil {
		return fmt.Errorf("create image assets directory: %w", err)
	}
	article := buildArticle(articleDate, opts.category, selected, now, images)
	if err := images.publishPicbed(ctx, articleDate, selected[0].repo); err != nil {
		return fmt.Errorf("publish images to GitHub picbed: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(article), 0o644); err != nil {
		return fmt.Errorf("write article: %w", err)
	}

	fmt.Printf("wrote article: %s\n", outputPath)
	fmt.Printf("primary repo: %s\n", selected[0].repo.FullName)
	return nil
}

func collectCandidates(ctx context.Context, client *gh.Client, opts options, now time.Time) ([]candidate, error) {
	recentRepos := loadRecentRepos(opts.outputDir, now, opts.recentRepoDays)
	if recentRepos.len() > 0 {
		fmt.Fprintf(os.Stderr, "excluding %d recent GitHub daily repos\n", recentRepos.len())
	}

	plans := buildSearchPlans(opts.category, opts.sinceDays, now, true)
	collected, err := searchCandidates(ctx, client, plans, now, recentRepos)
	if err != nil {
		return nil, err
	}
	if len(collected) == 0 {
		fmt.Fprintln(os.Stderr, "fresh GitHub search returned no candidates; retrying with a broader window")
		plans = buildSearchPlans(opts.category, opts.sinceDays*4, now, true)
		collected, err = searchCandidates(ctx, client, plans, now, recentRepos)
		if err != nil {
			return nil, err
		}
	}
	if len(collected) == 0 {
		fmt.Fprintln(os.Stderr, "broader GitHub search returned no candidates; retrying without a date qualifier")
		plans = buildSearchPlans(opts.category, opts.sinceDays, now, false)
		collected, err = searchCandidates(ctx, client, plans, now, recentRepos)
		if err != nil {
			return nil, err
		}
	}
	if len(collected) == 0 {
		return nil, fmt.Errorf("no GitHub candidates found")
	}

	sortCandidates(collected)
	enriched := enrichWithReadme(ctx, client, collected, now, 14)
	enriched = filterCandidates(enriched)
	sortCandidates(enriched)
	return enriched, nil
}

func searchCandidates(ctx context.Context, client *gh.Client, plans []searchPlan, now time.Time, recentRepos recentRepoSet) ([]candidate, error) {
	seen := map[string]bool{}
	candidates := []candidate{}
	var lastErr error

	for _, plan := range plans {
		items, err := searchRepositoriesWithRetry(ctx, client, plan)
		if err != nil {
			lastErr = err
			fmt.Fprintf(os.Stderr, "GitHub search skipped (%s): %v\n", plan.query, err)
			continue
		}
		for _, item := range items {
			if item.Fork || item.Archived || item.Disabled || item.FullName == "" || seen[item.FullName] {
				continue
			}
			if recentRepos.has(item.FullName) {
				fmt.Fprintf(os.Stderr, "GitHub candidate skipped because it was recently used: %s\n", item.FullName)
				continue
			}
			repo := gh.ToDomainRepository(item, item.Topics, item.Description)
			if !isUsefulProject(repo) {
				continue
			}
			score := scoring.ScoreRepository(repo, now)
			if score.Quality < 42 {
				continue
			}
			seen[item.FullName] = true
			candidates = append(candidates, candidate{
				response: item,
				repo:     repo,
				score:    score,
				source:   plan.query,
			})
		}
	}

	if len(candidates) == 0 && lastErr != nil {
		return nil, fmt.Errorf("github search unavailable after retries: %w", lastErr)
	}
	return candidates, nil
}

func loadRecentRepos(outputDir string, now time.Time, days int) recentRepoSet {
	result := recentRepoSet{
		fullNames: map[string]bool{},
		safeKeys:  map[string]bool{},
	}
	if strings.TrimSpace(outputDir) == "" || days <= 0 {
		return result
	}

	files, err := filepath.Glob(filepath.Join(outputDir, "*-github-daily.md"))
	if err == nil {
		for _, file := range files {
			if !isRecentArticlePath(file, now, days) {
				continue
			}
			if fullName := readPrimaryRepo(file); fullName != "" {
				result.addFullName(fullName)
			}
		}
	}

	assetDirs, err := filepath.Glob(filepath.Join(outputDir, "assets", "????-??-??-*"))
	if err == nil {
		for _, dir := range assetDirs {
			if !isRecentArticlePath(filepath.Base(dir), now, days) {
				continue
			}
			if key := recentAssetRepoKey(filepath.Base(dir)); key != "" {
				result.safeKeys[key] = true
			}
		}
	}
	return result
}

func (s recentRepoSet) addFullName(fullName string) {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return
	}
	s.fullNames[strings.ToLower(fullName)] = true
	owner, name, ok := strings.Cut(fullName, "/")
	if ok {
		s.safeKeys[safeRepoKey(owner, name)] = true
	}
}

func (s recentRepoSet) has(fullName string) bool {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return false
	}
	if s.fullNames[strings.ToLower(fullName)] {
		return true
	}
	owner, name, ok := strings.Cut(fullName, "/")
	return ok && s.safeKeys[safeRepoKey(owner, name)]
}

func (s recentRepoSet) len() int {
	seen := map[string]bool{}
	for fullName := range s.fullNames {
		seen[fullName] = true
	}
	for key := range s.safeKeys {
		seen["key:"+key] = true
	}
	return len(seen)
}

func isRecentArticlePath(pathValue string, now time.Time, days int) bool {
	name := filepath.Base(pathValue)
	if len(name) < len("2006-01-02") {
		return false
	}
	date, err := time.ParseInLocation("2006-01-02", name[:len("2006-01-02")], now.Location())
	if err != nil {
		return false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	oldest := today.AddDate(0, 0, -days)
	return !date.Before(oldest) && !date.After(today)
}

func readPrimaryRepo(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "primary_repo:") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "primary_repo:")), `"'`)
		}
		if line == "---" {
			continue
		}
		if strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") {
			break
		}
	}
	return ""
}

func recentAssetRepoKey(dirName string) string {
	const datePrefixLength = len("2006-01-02-")
	if len(dirName) <= datePrefixLength {
		return ""
	}
	return strings.Trim(strings.ToLower(dirName[datePrefixLength:]), "-")
}

func safeRepoKey(owner string, name string) string {
	return safeFilename(owner + "-" + name)
}

func searchRepositoriesWithRetry(ctx context.Context, client *gh.Client, plan searchPlan) ([]gh.RepositoryResponse, error) {
	const attempts = 3

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		items, _, err := client.SearchRepositoriesWithSort(ctx, plan.query, 18, plan.sort, plan.order)
		if err == nil {
			return items, nil
		}
		lastErr = err
		if !isRetryableGitHubSearchError(err) || attempt == attempts {
			break
		}
		delay := time.Duration(attempt) * time.Second
		fmt.Fprintf(os.Stderr, "GitHub search retrying (%s, attempt %d/%d): %v\n", plan.query, attempt+1, attempts, err)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func isRetryableGitHubSearchError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	text := strings.ToLower(err.Error())
	return containsAny(text, "no such host", "connection reset", "connection refused", "temporary failure", "eof")
}

func enrichWithReadme(ctx context.Context, client *gh.Client, candidates []candidate, now time.Time, limit int) []candidate {
	if len(candidates) == 0 {
		return candidates
	}
	if limit > len(candidates) {
		limit = len(candidates)
	}

	result := append([]candidate(nil), candidates...)
	for index := 0; index < limit; index++ {
		current := result[index]
		_, readme, _, err := client.GetReadme(ctx, current.repo.Owner, current.repo.Name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "README skipped (%s): %v\n", current.repo.FullName, err)
			continue
		}
		summary := editorial.SummarizeReadme(readme, current.repo.Description)
		current.repo = gh.ToDomainRepository(current.response, current.response.Topics, summary)
		current.score = scoring.ScoreRepository(current.repo, now)
		current.readme = readme
		current.imageURL = gh.ResolveReadmeIconURL(readme, current.repo.Owner, current.repo.Name, current.response.DefaultBranch)
		result[index] = current
	}
	return result
}

func filterCandidates(candidates []candidate) []candidate {
	filtered := make([]candidate, 0, len(candidates))
	for _, item := range candidates {
		if !isUsefulProject(item.repo) || item.score.Quality < 42 {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func buildSearchPlans(category string, sinceDays int, now time.Time, withDate bool) []searchPlan {
	category = normalizeCategory(category)
	since := now.AddDate(0, 0, -sinceDays).Format("2006-01-02")
	base := "stars:80..20000 fork:false archived:false"
	createdBase := "stars:30..8000 fork:false archived:false"
	if withDate {
		base += " pushed:>" + since
		createdBase += " created:>" + since
	}

	topics := topicsForCategory(category)
	plans := make([]searchPlan, 0, len(topics)+1)
	for _, topic := range topics {
		plans = append(plans, searchPlan{
			query: fmt.Sprintf("%s topic:%s", base, topic),
			sort:  "updated",
			order: "desc",
		})
	}
	plans = append(plans, searchPlan{
		query: createdBase + " " + createdKeywordsForCategory(category),
		sort:  "stars",
		order: "desc",
	})
	if len(plans) > 7 {
		plans = plans[:7]
	}
	return plans
}

func topicsForCategory(category string) []string {
	switch normalizeCategory(category) {
	case "ai":
		return []string{"ai", "llm", "artificial-intelligence", "agents", "mcp"}
	case "devtools":
		return []string{"developer-tools", "cli", "productivity", "automation", "observability"}
	case "web":
		return []string{"web", "react", "nextjs", "typescript", "frontend"}
	case "data":
		return []string{"database", "analytics", "data-engineering", "postgresql", "vector-database"}
	case "backend":
		return []string{"backend", "api", "devops", "kubernetes", "serverless"}
	default:
		return []string{"ai", "developer-tools", "automation", "productivity", "database", "web"}
	}
}

func createdKeywordsForCategory(category string) string {
	switch normalizeCategory(category) {
	case "ai":
		return "ai OR llm"
	case "devtools":
		return "cli"
	case "web":
		return "typescript"
	case "data":
		return "database"
	case "backend":
		return "api"
	default:
		return "tool"
	}
}

func selectCandidates(candidates []candidate, limit int) []candidate {
	if len(candidates) <= limit {
		return candidates
	}

	selected := []candidate{}
	usedOwners := map[string]bool{}
	languageCounts := map[string]int{}
	for _, item := range candidates {
		language := strings.ToLower(strings.TrimSpace(item.repo.Language))
		if usedOwners[strings.ToLower(item.repo.Owner)] {
			continue
		}
		if language != "" && languageCounts[language] >= 2 && len(selected) >= 3 {
			continue
		}
		selected = append(selected, item)
		usedOwners[strings.ToLower(item.repo.Owner)] = true
		if language != "" {
			languageCounts[language]++
		}
		if len(selected) >= limit {
			return selected
		}
	}
	for _, item := range candidates {
		if len(selected) >= limit {
			break
		}
		if containsCandidate(selected, item.repo.FullName) {
			continue
		}
		selected = append(selected, item)
	}
	return selected
}

func sortCandidates(candidates []candidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidateRank(candidates[i])
		right := candidateRank(candidates[j])
		if left == right {
			return candidates[i].repo.Stars > candidates[j].repo.Stars
		}
		return left > right
	})
}

func candidateRank(item candidate) float64 {
	repo := item.repo
	score := item.score
	rank := float64(score.Quality)*3.0 +
		float64(score.Novelty)*1.25 +
		float64(score.Freshness)*1.1 +
		float64(score.Documentation)*0.7 +
		math.Min(float64(repo.Stars), 8000)/120.0 +
		math.Min(float64(repo.Forks), 1200)/90.0

	if repo.Stars > 30000 {
		rank -= 80
	}
	if strings.TrimSpace(item.readme) != "" {
		rank += 12
	}
	return rank
}

func buildArticle(date string, category string, selected []candidate, now time.Time, images *imageStore) string {
	primary := selected[0]
	repo := primary.repo
	title := articleTitle(primary)

	var builder strings.Builder
	builder.WriteString("---\n")
	builder.WriteString("title: " + quoteYAML(title) + "\n")
	builder.WriteString("date: " + date + "\n")
	builder.WriteString("category: github\n")
	builder.WriteString("source: github-api\n")
	builder.WriteString("primary_repo: " + repo.FullName + "\n")
	builder.WriteString("---\n\n")

	builder.WriteString("# " + title + "\n\n")
	builder.WriteString("这次看到的是这个项目：\n\n")
	builder.WriteString(fmt.Sprintf("[%s](%s)\n\n", repo.FullName, repo.HTMLURL))
	builder.WriteString(markdownImage(images, "GitHub 项目预览", repoPreviewImage(date, repo, "main")))
	builder.WriteString("\n")
	if image := heroImage(primary); image != "" {
		builder.WriteString(markdownImage(images, repo.Name+" 项目图", image))
		builder.WriteString("\n")
	}
	builder.WriteString(repoBadges(repo))
	builder.WriteString("\n")
	if image := projectExtraImage(primary); image != "" {
		builder.WriteString(markdownImage(images, repo.Name+" 截图", image))
		builder.WriteString("\n")
	}
	for index, image := range projectReadmeImages(primary, 8) {
		builder.WriteString(markdownImage(images, fmt.Sprintf("%s 项目图 %d", repo.Name, index+1), image))
		builder.WriteString("\n")
	}

	themes := articleThemes(primary, now)
	for index, theme := range themes {
		builder.WriteString(fmt.Sprintf("## %d. %s\n\n", index+1, theme.title))
		builder.WriteString(theme.body + "\n\n")
		if theme.imageURL != "" {
			builder.WriteString(markdownImage(images, theme.imageAlt, theme.imageURL))
			builder.WriteString("\n")
		}
	}
	builder.WriteString(markdownImage(images, "项目卡片", repoPreviewImage(date, repo, "card")))
	builder.WriteString("\n")

	builder.WriteString("今天就先聊到这里。\n")
	return builder.String()
}

func articleTitle(item candidate) string {
	repo := item.repo
	return fmt.Sprintf("今天 Github 看到一个%s：%s", headlineScenario(repo), repo.Name)
}

func headlineScenario(repo domain.Repository) string {
	text := repoText(repo)
	switch {
	case containsAny(text, "clawx", "openclaw", "desktop interface", "desktop app"):
		return "把 AI Agent 装进桌面的项目"
	case containsAny(text, "woodpecker", "continuous integration", "continuous delivery", "ci/cd", "ci-cd", "build pipeline", "pipelines"):
		return "帮团队自建 CI/CD 的项目"
	case containsAny(text, "terragrunt", "terraform", "opentofu", "infrastructure as code", "iac"):
		return "把基础设施代码管得更顺的项目"
	case containsAny(text, "starrocks", "database", "postgres", "analytics", "warehouse", "lakehouse", "query engine", "big-data", "olap"):
		return "让实时分析跑得更快的数据库"
	case containsAny(text, "crawler", "crawlee", "scraping", "web scraping", "browser automation"):
		return "做网页采集和浏览器自动化的项目"
	case containsAnyToken(text, "rag", "knowledge", "vector", "embedding", "retrieval"):
		return "把知识库变成问答助手的项目"
	case containsAny(text, "mcp", "agent", "tool-use", "function calling"):
		return "让 AI Agent 更好用的项目"
	case containsAny(text, "prompt", "workflow", "automation", "template"):
		return "管理提示词和自动化流程的项目"
	case containsAny(text, "observability", "monitoring", "eval", "tracing"):
		return "给 AI 应用做观测和评测的项目"
	case containsAny(text, "ui", "react", "nextjs", "frontend", "component"):
		return "更快做产品界面的项目"
	case containsAny(text, "cli", "developer tool", "devtools", "terminal"):
		return "改善开发体验的小工具"
	default:
		return "解决具体问题的开源项目"
	}
}

func markdownImage(images *imageStore, alt string, src string) string {
	if strings.TrimSpace(src) == "" {
		return ""
	}
	if images != nil {
		src = images.localize(src, alt)
	}
	if strings.TrimSpace(src) == "" {
		return ""
	}
	return fmt.Sprintf("![%s](%s)\n", strings.TrimSpace(alt), strings.TrimSpace(src))
}

func newImageStore(opts options, date string, repo domain.Repository) (*imageStore, error) {
	name := safeFilename(date + "-" + repo.Owner + "-" + repo.Name)
	relDir := filepath.ToSlash(filepath.Join("assets", name))
	assetDir := filepath.Join(opts.outputDir, relDir)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		return nil, err
	}
	imageBedDir := strings.TrimSpace(opts.imageBedDir)
	if imageBedDir != "" {
		if err := os.MkdirAll(imageBedDir, 0o755); err != nil {
			return nil, err
		}
	}
	return &imageStore{
		assetDir:        assetDir,
		relDir:          relDir,
		imageBedDir:     imageBedDir,
		imageBedBaseURL: strings.TrimRight(strings.TrimSpace(opts.imageBedBaseURL), "/"),
		imageBedPath:    filepath.ToSlash(strings.Trim(strings.TrimSpace(opts.imageBedPath), "/")),
		imageBedPrefix:  safeFilename("github-" + strings.ReplaceAll(date, "-", "") + "-" + repo.Owner + "-" + repo.Name),
		imageBedPush:    opts.imageBedPush,
		seen:            map[string]string{},
		client:          &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (s *imageStore) localize(src string, alt string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return ""
	}
	if cached, ok := s.seen[src]; ok {
		return cached
	}

	request, err := http.NewRequest(http.MethodGet, src, nil)
	if err != nil {
		return ""
	}
	request.Header.Set("User-Agent", "PreHub-DailyArticle/0.1")
	response, err := s.client.Do(request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "image download skipped (%s): %v\n", src, err)
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "image download skipped (%s): status %d\n", src, response.StatusCode)
		return ""
	}

	s.index++
	extension := imageExtension(src, response.Header.Get("content-type"))
	filename := fmt.Sprintf("%02d-%s%s", s.index, safeFilename(alt), extension)
	target := filepath.Join(s.assetDir, filename)
	file, err := os.Create(target)
	if err != nil {
		return ""
	}
	written, err := io.Copy(file, io.LimitReader(response.Body, 15<<20))
	closeErr := file.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "image save skipped (%s): %v\n", src, err)
		_ = os.Remove(target)
		return ""
	}
	if closeErr != nil {
		fmt.Fprintf(os.Stderr, "image save skipped (%s): %v\n", src, closeErr)
		_ = os.Remove(target)
		return ""
	}
	if written < 1024 {
		_ = os.Remove(target)
		return ""
	}

	rel := filepath.ToSlash(filepath.Join(s.relDir, filename))
	result := rel
	if picbedURL, err := s.copyToPicbed(target, filename); err == nil && picbedURL != "" {
		result = picbedURL
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "picbed copy skipped (%s): %v\n", target, err)
	}
	s.seen[src] = result
	return result
}

func (s *imageStore) copyToPicbed(localPath string, filename string) (string, error) {
	if s.imageBedDir == "" || s.imageBedBaseURL == "" {
		return "", nil
	}
	targetName := s.imageBedPrefix + "-" + filename
	targetPath := filepath.Join(s.imageBedDir, filepath.FromSlash(s.imageBedPath), targetName)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return "", err
	}
	if err := copyFile(localPath, targetPath); err != nil {
		return "", err
	}
	repoPath := filepath.ToSlash(filepath.Join(s.imageBedPath, targetName))
	s.imageBedFiles = append(s.imageBedFiles, repoPath)
	return s.imageBedBaseURL + "/" + escapeURLPath(repoPath), nil
}

func (s *imageStore) publishPicbed(ctx context.Context, date string, repo domain.Repository) error {
	if s.imageBedDir == "" || !s.imageBedPush || len(s.imageBedFiles) == 0 {
		return nil
	}
	files := uniqueStrings(s.imageBedFiles)
	status, err := gitOutput(ctx, s.imageBedDir, append([]string{"status", "--short", "--"}, files...)...)
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		return nil
	}
	if _, err := gitOutput(ctx, s.imageBedDir, append([]string{"add", "--"}, files...)...); err != nil {
		return err
	}
	message := fmt.Sprintf("Add GitHub daily images %s %s", date, repo.Name)
	if _, err := gitOutput(ctx, s.imageBedDir, append([]string{"commit", "-m", message, "--"}, files...)...); err != nil {
		return err
	}
	if _, err := gitOutput(ctx, s.imageBedDir, "push"); err != nil {
		return err
	}
	return nil
}

func copyFile(source string, target string) error {
	input, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(target, input, 0o644)
}

func escapeURLPath(value string) string {
	parts := strings.Split(filepath.ToSlash(value), "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func imageExtension(src string, contentType string) string {
	if parsed, err := url.Parse(src); err == nil {
		if extension := strings.ToLower(filepath.Ext(parsed.Path)); extension != "" && len(extension) <= 6 {
			return extension
		}
	}
	contentType = strings.ToLower(strings.Split(contentType, ";")[0])
	switch contentType {
	case "image/svg+xml":
		return ".svg"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func safeFilename(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = filenamePattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-._")
	if value == "" {
		return "image"
	}
	if len(value) > 48 {
		value = value[:48]
		value = strings.Trim(value, "-._")
	}
	return value
}

func heroImage(item candidate) string {
	if strings.TrimSpace(item.imageURL) != "" {
		return item.imageURL
	}
	if strings.TrimSpace(item.repo.AvatarURL) != "" {
		return item.repo.AvatarURL
	}
	return repoOwnerAvatar(item.repo)
}

func projectExtraImage(item candidate) string {
	if strings.EqualFold(item.repo.FullName, "woodpecker-ci/woodpecker") {
		return "https://raw.githubusercontent.com/woodpecker-ci/woodpecker/main/docs/woodpecker.png"
	}
	if strings.EqualFold(item.repo.FullName, "StarRocks/starrocks") {
		return "https://docs.starrocks.io/img/logo.svg"
	}
	return ""
}

func projectReadmeImages(item candidate, limit int) []string {
	if strings.TrimSpace(item.readme) == "" || limit <= 0 {
		return nil
	}
	images := extractReadmeImageURLs(item.readme, item.repo.Owner, item.repo.Name, item.response.DefaultBranch)
	result := []string{}
	seen := map[string]bool{}
	for _, image := range images {
		if image == "" || seen[image] || image == item.imageURL || image == item.repo.AvatarURL {
			continue
		}
		if image == projectExtraImage(item) {
			continue
		}
		seen[image] = true
		result = append(result, image)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func extractReadmeImageURLs(readme string, owner string, repo string, defaultBranch string) []string {
	candidates := []string{}
	for _, match := range markdownImagePattern.FindAllStringSubmatch(readme, -1) {
		if len(match) < 3 {
			continue
		}
		if resolved := resolveReadmeAssetURL(match[2], owner, repo, defaultBranch); shouldUseArticleImage(resolved) {
			candidates = append(candidates, resolved)
		}
	}

	for _, tag := range htmlImagePattern.FindAllString(readme, -1) {
		attrs := map[string]string{}
		for _, match := range htmlAttributePattern.FindAllStringSubmatch(tag, -1) {
			if len(match) < 3 {
				continue
			}
			attrs[strings.ToLower(match[1])] = strings.TrimSpace(match[2])
		}
		if resolved := resolveReadmeAssetURL(attrs["src"], owner, repo, defaultBranch); shouldUseArticleImage(resolved) {
			candidates = append(candidates, resolved)
		}
	}
	return candidates
}

func shouldUseArticleImage(src string) bool {
	value := strings.ToLower(strings.TrimSpace(src))
	if value == "" {
		return false
	}
	if containsAny(value,
		"shields.io",
		"badgen.net",
		"badge",
		"codecov",
		"goreportcard",
		"pkg.go.dev/badge",
		"matrix.org",
		"opencollective.com",
		"star-history.com",
		"api.star-history.com",
		"bestpractices.coreinfrastructure.org",
		"results.pre-commit.ci",
		"translate.",
	) {
		return false
	}
	return containsAny(value, ".png", ".jpg", ".jpeg", ".webp", ".gif", ".svg")
}

func resolveReadmeAssetURL(src string, owner string, repo string, defaultBranch string) string {
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
		if strings.EqualFold(parsed.Host, "github.com") {
			segments := strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/")
			if len(segments) >= 5 && segments[2] == "blob" {
				return "https://raw.githubusercontent.com/" + url.PathEscape(segments[0]) + "/" + url.PathEscape(segments[1]) + "/" + url.PathEscape(segments[3]) + "/" + escapePathSegments(strings.Join(segments[4:], "/"))
			}
		}
		return parsed.String()
	}
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	cleanPath := strings.TrimPrefix(src, "./")
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	if cleanPath == "" || strings.HasPrefix(cleanPath, "../") {
		return ""
	}
	assetPath := strings.TrimPrefix(path.Clean(cleanPath), "/")
	return "https://raw.githubusercontent.com/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/" + url.PathEscape(defaultBranch) + "/" + escapePathSegments(assetPath)
}

func escapePathSegments(value string) string {
	segments := strings.Split(value, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

func articleThemes(item candidate, now time.Time) []articleTheme {
	if strings.EqualFold(item.repo.FullName, "woodpecker-ci/woodpecker") {
		return woodpeckerThemes(item, now)
	}
	if strings.EqualFold(item.repo.FullName, "StarRocks/starrocks") {
		return starRocksThemes(item, now)
	}
	return genericThemes(item, now)
}

func articleSources(repo domain.Repository) []string {
	sources := []string{
		"GitHub Repository Search API",
		"仓库 README 和公开元数据",
	}
	switch {
	case strings.EqualFold(repo.FullName, "StarRocks/starrocks"):
		sources = append(sources,
			"StarRocks 官方文档：https://docs.starrocks.io/",
			"StarRocks GitHub：https://github.com/StarRocks/starrocks",
		)
	case strings.EqualFold(repo.FullName, "woodpecker-ci/woodpecker"):
		sources = append(sources,
			"Woodpecker 官方文档：https://woodpecker-ci.org/docs/intro",
			"Woodpecker GitHub：https://github.com/woodpecker-ci/woodpecker",
		)
	}
	return sources
}

func woodpeckerThemes(item candidate, now time.Time) []articleTheme {
	repo := item.repo
	return []articleTheme{
		{
			title: "它到底是什么",
			body:  "Woodpecker 是一个 CI/CD 工具。说人话就是：代码一提交，它可以自动跑测试、打包、构建镜像、部署应用。它自己的定位也很直接：轻量、简单、快。",
		},
		{
			title:    "它解决什么麻烦",
			body:     "很多小团队不想维护一套很重的 Jenkins，也不想把所有流水线都绑死在某个平台里。Woodpecker 走的是更轻的路子：用 pipeline 把测试、构建、部署这些步骤串起来，流程清楚，迁移成本也低一些。",
			imageURL: repoPreviewImage(now.Format("2006-01-02"), repo, "pipeline"),
			imageAlt: "Woodpecker pipeline 预览",
		},
		{
			title: "核心看点",
			body:  "它有插件体系。构建镜像、发通知、部署服务，这些动作都可以靠插件接起来。官方还做了插件列表页面，方便直接找现成工具。对团队来说，这比自己到处拼脚本舒服很多。",
		},
		{
			title: "为什么值得看",
			body:  fmt.Sprintf("它现在有 %s stars、%s forks，轻量是一个很大的信号。README 里也写得很实在：默认可以用 SQLite，空闲时 Server 大约 100MB 内存，Agent 大约 30MB 内存。也就是说，它不是那种一上来就要一堆机器伺候的大家伙。", formatInt(repo.Stars), formatInt(repo.Forks)),
		},
		{
			title: "怎么用起来",
			body:  "它可以自己部署。先拿一个小仓库试，把测试、构建、部署拆成几步 pipeline。跑通之后，再接密钥、镜像仓库和通知插件。",
		},
		{
			title: "适合谁，以及先注意什么",
			body:  "适合想自建 CI/CD，又不想维护重型平台的团队。它再轻，也还是一套 CI/CD 系统。真要接入团队项目，先看能不能连代码托管平台、能不能安全管理密钥、插件够不够用、失败日志好不好排查。",
		},
	}
}

func starRocksThemes(item candidate, now time.Time) []articleTheme {
	repo := item.repo
	return []articleTheme{
		{
			title: "它到底是什么",
			body:  "StarRocks 是一个开源的分析型数据库，也可以理解成一个很快的 SQL 查询引擎。它主打的是实时分析、即席查询和湖仓查询。官方定位很直：让企业做实时分析更简单。",
		},
		{
			title:    "它解决什么麻烦",
			body:     "很多公司数据越来越多，问题却很朴素：报表慢、临时查询慢、实时数据进来了但分析跟不上。StarRocks 解决的就是这类场景。它想让新鲜数据也能很快被查出来，而不是等很久才进报表。",
			imageURL: repoPreviewImage(now.Format("2006-01-02"), repo, "analytics"),
			imageAlt: "StarRocks 分析场景",
		},
		{
			title: "核心看点",
			body:  "它的关键词有几个：MPP、向量化执行、列式存储、CBO 优化器、物化视图。听着有点数据库味，但你可以简单理解：把大查询拆开并行跑，尽量吃满 CPU，再自动挑一个更聪明的执行计划。",
		},
		{
			title: "为什么值得看",
			body:  "StarRocks 不只查自己库里的数据。它也能直接查 Hive、Iceberg、Delta Lake、Hudi 这些数据湖里的数据。很多团队最怕搬数据，搬来搬去又慢又容易出错。能直接查外部数据，就少了一层麻烦。",
		},
		{
			title: "怎么用起来",
			body:  "别一上来就替换核心数仓。更现实的玩法是：先拿一个慢报表、一个实时看板，或者一个数据湖查询场景试一下。它兼容 MySQL 协议，很多 BI 工具和 MySQL 客户端可以直接连，这点对上手很友好。",
		},
		{
			title: "适合谁，以及先注意什么",
			body:  fmt.Sprintf("适合做数据平台、实时看板、用户行为分析、湖仓分析的团队。它现在有 %s stars、%s forks，Apache-2.0 协议，最近也很活跃。但它毕竟是数据库，不是小工具。真正上生产前，要认真压测、看资源成本、看运维复杂度，也要看你团队有没有数据库维护经验。", formatInt(repo.Stars), formatInt(repo.Forks)),
		},
	}
}

func genericThemes(item candidate, now time.Time) []articleTheme {
	repo := item.repo
	return []articleTheme{
		{
			title: "它到底是什么",
			body:  fmt.Sprintf("简单说，%s 是一个用来%s的开源项目。它不是泛泛而谈的大平台，至少从仓库信息看，目标比较具体。", repo.Name, plainScenario(repo)),
		},
		{
			title: "它解决什么麻烦",
			body:  fmt.Sprintf("它切中的问题是：%s。对普通团队来说，这类项目最大的价值不是炫技，而是少走一点重复造轮子的路。", productScenario(repo)),
		},
		{
			title: "核心看点",
			body:  fmt.Sprintf("它的公开信号还不错：%s stars、%s forks，主要语言是 %s，最近更新是 %s。至少说明有人在看，也有人在维护。", formatInt(repo.Stars), formatInt(repo.Forks), valueOr(repo.Language, "未标注"), formatPushedAt(repo.PushedAt, now)),
		},
		{
			title: "为什么值得看",
			body:  fmt.Sprintf("这类项目的价值在于省时间。遇到%s这种问题时，先看一个维护中的开源实现，往往比自己从零拼一套更现实。", productScenario(repo)),
		},
		{
			title: "怎么用起来",
			body:  "先别急着接生产。最简单的办法是打开 README，找 quickstart 或 example，拿一个小场景跑起来。能跑通，再看配置、部署和权限这些细节。",
		},
		{
			title: "适合谁，以及先注意什么",
			body:  targetUsers(repo) + "。如果只是个人尝鲜，也可以先收藏，等遇到类似问题时再翻出来试。" + shortCaveat(repo) + "开源项目看起来很香，但最好先跑 demo。能跑起来，再谈接入。",
		},
	}
}

func repoPreviewImage(date string, repo domain.Repository, suffix string) string {
	seed := "prehub-" + strings.ReplaceAll(date, "-", "") + "-" + suffix
	return "https://opengraph.githubassets.com/" + url.PathEscape(seed) + "/" + url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name)
}

func repoOwnerAvatar(repo domain.Repository) string {
	if strings.TrimSpace(repo.Owner) == "" {
		return ""
	}
	return "https://github.com/" + url.PathEscape(repo.Owner) + ".png?size=240"
}

func repoBadges(repo domain.Repository) string {
	if strings.TrimSpace(repo.Owner) == "" || strings.TrimSpace(repo.Name) == "" {
		return ""
	}
	return fmt.Sprintf("> Stars：%s ｜ Forks：%s ｜ License：%s\n", formatInt(repo.Stars), formatInt(repo.Forks), valueOr(repo.License, "unknown"))
}

func plainScenario(repo domain.Repository) string {
	scenario := productScenario(repo)
	replacements := []struct {
		old string
		new string
	}{
		{"搭建自己的自动化构建、测试和部署流程", "搭建 CI/CD 自动化流程"},
		{"管理 Terraform / OpenTofu 基础设施代码和多环境配置", "管理 Terraform 多环境配置"},
		{"提升数据查询、实时分析和湖仓链路的工程效率", "做更快的数据查询和实时分析"},
		{"构建可复用的数据采集和浏览器自动化能力", "做网页采集和浏览器自动化"},
		{"搭建安全可控的团队沟通与协作基础设施", "搭建自己的团队聊天和协作系统"},
		{"让 AI agent 更稳定地调用工具和完成任务", "让 AI Agent 调工具、跑任务"},
		{"把文档、知识库和数据变成可检索、可追问的产品能力", "把文档和数据做成可搜索、可问答的工具"},
		{"把重复的提示词和工作流沉淀成可复用资产", "管理提示词和自动化流程"},
		{"补齐系统上线后的观测、评测和调试闭环", "看清系统运行状态，方便排查问题"},
		{"加快前端产品界面和交互体验的交付", "更快做前端页面和交互"},
		{"改善开发者日常操作和本地工具效率", "提高开发者本地工具效率"},
	}
	for _, replacement := range replacements {
		if scenario == replacement.old {
			return replacement.new
		}
	}
	return "解决一个具体的技术问题"
}

func shortCaveat(repo domain.Repository) string {
	parts := []string{}
	license := strings.ToLower(strings.TrimSpace(repo.License))
	if license == "" || license == "unknown" || license == "other" {
		parts = append(parts, "协议不够清楚，商用前要看一下 license。")
	}
	if repo.Stars < 300 {
		parts = append(parts, "项目还比较早，适合先玩一玩，不要直接放核心业务。")
	}
	if daysSincePushed(repo.PushedAt) > 90 {
		parts = append(parts, "最近更新不算频繁，要看 issue 里有没有人维护。")
	}
	if len(parts) == 0 {
		return "没什么神秘的，先看 README，再跑一下 quickstart。"
	}
	return strings.Join(parts, "")
}

func isUsefulProject(repo domain.Repository) bool {
	text := strings.ToLower(strings.Join([]string{
		repo.FullName,
		repo.Description,
		repo.Summary,
		strings.Join(repo.Topics, " "),
	}, " "))
	name := strings.ToLower(repo.Name)
	blocked := []string{
		"awesome", "curated-list", "free-api", "api-key", "leaked", "crack",
		"wallpaper", "interview-questions", "leetcode", "demo repo",
		"benchmark", "benchmarks", "benchmarking", "which is the fastest",
		"performance comparison", "performance test", "framework comparison",
	}
	for _, term := range blocked {
		if strings.Contains(name, term) || strings.Contains(text, term) {
			return false
		}
	}
	return strings.TrimSpace(repo.Description) != ""
}

func pmRecommendationReason(repo domain.Repository, score domain.ScoreBreakdown) string {
	return fmt.Sprintf(
		"从产品经理视角看，它值得关注的不是代码量，而是把「%s」包装成了一个可试用、可比较、可落地的方案。当前公开信号是 %s stars、%s forks、%s；这足够支持一次小范围验证，但还不等于可以直接生产采用。",
		productScenario(repo),
		formatInt(repo.Stars),
		formatInt(repo.Forks),
		scoreSignal(score),
	)
}

func scoreSignal(score domain.ScoreBreakdown) string {
	parts := []string{}
	if score.Freshness >= 70 {
		parts = append(parts, "近期维护活跃")
	}
	if score.Documentation >= 70 {
		parts = append(parts, "文档信息较完整")
	}
	if score.License >= 70 {
		parts = append(parts, "授权边界较清晰")
	}
	if len(parts) == 0 {
		return "仍需要补充验证"
	}
	return strings.Join(parts, "，")
}

func recommendationOpening(repo domain.Repository) string {
	return fmt.Sprintf("它的价值不只是「有代码」，而是把「%s」这个场景做成了一个可以直接评估的开源方案。", productScenario(repo))
}

func shortValueLine(repo domain.Repository) string {
	scenario := productScenario(repo)
	if len([]rune(scenario)) > 32 {
		scenario = string([]rune(scenario)[:32]) + "..."
	}
	return scenario
}

func productScenario(repo domain.Repository) string {
	text := repoText(repo)
	switch {
	case containsAny(text, "woodpecker", "continuous integration", "continuous delivery", "ci/cd", "ci-cd", "build pipeline", "pipelines"):
		return "搭建自己的自动化构建、测试和部署流程"
	case containsAny(text, "terragrunt", "terraform", "opentofu", "infrastructure as code", "iac"):
		return "管理 Terraform / OpenTofu 基础设施代码和多环境配置"
	case containsAny(text, "database", "postgres", "analytics", "warehouse", "lakehouse", "query engine", "big-data", "olap"):
		return "提升数据查询、实时分析和湖仓链路的工程效率"
	case containsAny(text, "crawler", "crawlee", "scraping", "web scraping", "browser automation"):
		return "构建可复用的数据采集和浏览器自动化能力"
	case containsAny(text, "matrix", "communication", "chat", "messaging"):
		return "搭建安全可控的团队沟通与协作基础设施"
	case containsAny(text, "mcp", "agent", "tool-use", "function calling"):
		return "让 AI agent 更稳定地调用工具和完成任务"
	case containsAnyToken(text, "rag", "knowledge", "vector", "embedding", "retrieval"):
		return "把文档、知识库和数据变成可检索、可追问的产品能力"
	case containsAny(text, "prompt", "workflow", "automation", "template"):
		return "把重复的提示词和工作流沉淀成可复用资产"
	case containsAny(text, "observability", "monitoring", "eval", "tracing"):
		return "补齐系统上线后的观测、评测和调试闭环"
	case containsAny(text, "ui", "react", "nextjs", "frontend", "component"):
		return "加快前端产品界面和交互体验的交付"
	case containsAny(text, "cli", "developer tool", "devtools", "terminal"):
		return "改善开发者日常操作和本地工具效率"
	default:
		return "把一个明确的技术问题包装成可试用的开源产品"
	}
}

func targetUsers(repo domain.Repository) string {
	text := repoText(repo)
	switch {
	case containsAny(text, "woodpecker", "continuous integration", "continuous delivery", "ci/cd", "ci-cd", "build pipeline", "pipelines"):
		return "需要自动化构建、测试、部署流程的开发团队"
	case containsAny(text, "terragrunt", "terraform", "opentofu", "infrastructure as code", "iac"):
		return "使用 Terraform 或 OpenTofu 管理基础设施的开发和运维团队"
	case containsAny(text, "database", "analytics", "postgres", "data"):
		return "需要搭建数据产品、分析平台或内部数据工具的团队"
	case containsAny(text, "crawler", "scraping", "browser automation"):
		return "需要采集公开数据、搭建自动化运营或内容分析流程的团队"
	case containsAny(text, "matrix", "communication", "chat", "messaging"):
		return "需要私有化协作、社区运营或安全通信基础设施的团队"
	case containsAny(text, "react", "nextjs", "frontend", "ui"):
		return "需要快速验证前端体验、组件体系或 Web 工具的团队"
	case containsAny(text, "cli", "devtools", "developer"):
		return "重视研发效率、命令行体验和工程协作的开发团队"
	case containsAny(text, "agent", "mcp", "llm"):
		return "正在做 AI agent、内部自动化或大模型工具链的产品和研发团队"
	default:
		return "想用开源方案快速验证新功能、新流程或新工具的产品团队"
	}
}

func adoptionSuggestion(repo domain.Repository) string {
	if repo.Stars < 300 {
		return "先作为早期方案做小范围试跑，不建议直接进入关键生产链路"
	}
	if repo.Stars < 1500 {
		return "适合拿一个边缘场景做 PoC，重点看 quickstart、issue 和维护节奏"
	}
	return "可以进入正式选型清单，但仍要补一轮 license、部署成本和安全评估"
}

func demandSignal(repo domain.Repository) string {
	return fmt.Sprintf("它对应的不是单点炫技，而是「%s」这种会反复出现的真实需求。PM 可以先判断团队里是否已经有手工流程、脚本堆叠或跨工具复制粘贴的问题。", productScenario(repo))
}

func productBoundary(repo domain.Repository) string {
	if len(repo.Topics) >= 3 {
		return "从 topics 和 README 看，项目边界相对清楚，适合用一条主流程验证，而不是一上来就比较所有竞品。"
	}
	return "公开标签还不算丰富，建议先从 README 的核心用例倒推边界，避免把它误判成全能型平台。"
}

func growthSignal(repo domain.Repository, score domain.ScoreBreakdown) string {
	parts := []string{}
	if repo.Stars > 0 {
		parts = append(parts, formatInt(repo.Stars)+" stars")
	}
	if repo.Forks > 0 {
		parts = append(parts, formatInt(repo.Forks)+" forks")
	}
	if score.Freshness >= 70 {
		parts = append(parts, "近期仍活跃")
	}
	if score.License >= 70 {
		parts = append(parts, "授权边界相对清晰")
	}
	if len(parts) == 0 {
		return "当前公开信号还偏少，更适合作为观察样本。"
	}
	return strings.Join(parts, "，") + "。这些信号不能直接等同于产品成熟，但足够支持一次低成本验证。"
}

func validationPlan(repo domain.Repository) string {
	return "用 30-60 分钟跑通 README quickstart，再拿一个真实小场景验证输入、输出、失败提示和集成成本；如果要进团队工具链，再补安全、权限、数据流和维护节奏检查。"
}

func alternativeReason(repo domain.Repository) string {
	return fmt.Sprintf("%s；当前有 %s stars，适合%s。", productScenario(repo), formatInt(repo.Stars), targetUsers(repo))
}

func recommendationLevel(score domain.ScoreBreakdown) string {
	switch {
	case score.Quality >= 78:
		return "高，值得今天重点试用"
	case score.Quality >= 62:
		return "中高，适合进入选型清单"
	default:
		return "观察，适合先收藏再验证"
	}
}

func projectProblemParagraph(repo domain.Repository) string {
	return fmt.Sprintf(
		"从仓库描述和 README 信号看，%s 的核心切入点是「%s」。这类项目对产品团队的价值在于：不用从零搭一套能力，先用开源实现验证真实场景、交付成本和团队采用门槛，再决定是否继续投入。",
		repo.Name,
		productScenario(repo),
	)
}

func formatPushedAt(pushedAt string, now time.Time) string {
	if strings.TrimSpace(pushedAt) == "" {
		return "未知"
	}
	parsed, err := time.Parse(time.RFC3339, pushedAt)
	if err != nil {
		return pushedAt
	}
	days := int(now.Sub(parsed).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return fmt.Sprintf("%s（约 %d 天前）", parsed.Format("2006-01-02"), days)
}

func daysSincePushed(pushedAt string) float64 {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(pushedAt))
	if err != nil {
		return 9999
	}
	return time.Since(parsed).Hours() / 24
}

func formatTopics(topics []string) string {
	if len(topics) == 0 {
		return "未标注"
	}
	if len(topics) > 8 {
		topics = topics[:8]
	}
	return strings.Join(topics, "、")
}

func formatInt(value int) string {
	raw := strconv.Itoa(value)
	if len(raw) <= 3 {
		return raw
	}
	var parts []string
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	return strings.Join(parts, ",")
}

func categoryName(category string) string {
	switch normalizeCategory(category) {
	case "ai":
		return "AI 与 Agent 工具"
	case "devtools":
		return "开发者工具"
	case "web":
		return "Web 与前端体验"
	case "data":
		return "数据产品与数据库"
	case "backend":
		return "后端与基础设施"
	default:
		return "开源产品机会"
	}
}

func normalizeCategory(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "ai", "devtools", "web", "data", "backend":
		return strings.ToLower(strings.TrimSpace(category))
	default:
		return defaultCategory
	}
}

func repoText(repo domain.Repository) string {
	return strings.ToLower(strings.Join([]string{
		repo.FullName,
		repo.Description,
		repo.Summary,
		repo.Language,
		strings.Join(repo.Topics, " "),
	}, " "))
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func containsAnyToken(value string, needles ...string) bool {
	tokens := map[string]bool{}
	for _, token := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		if token != "" {
			tokens[token] = true
		}
	}
	for _, needle := range needles {
		if tokens[strings.ToLower(needle)] {
			return true
		}
	}
	return false
}

func containsCandidate(candidates []candidate, fullName string) bool {
	for _, item := range candidates {
		if strings.EqualFold(item.repo.FullName, fullName) {
			return true
		}
	}
	return false
}

func valueOr(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func quoteYAML(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func loadLocalEnv() map[string]string {
	values := map[string]string{}
	cwd, err := os.Getwd()
	if err != nil {
		return values
	}

	paths := []string{
		filepath.Join(cwd, ".env"),
		filepath.Join(cwd, ".env.local"),
	}
	if filepath.Base(cwd) == "backend" {
		paths = append(paths,
			filepath.Join(cwd, "..", ".env"),
			filepath.Join(cwd, "..", ".env.local"),
		)
	}
	for _, path := range paths {
		readEnvFile(path, values)
	}
	return values
}

func readEnvFile(path string, values map[string]string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		values[key] = value
	}
}

func envFrom(key string, fallback string, values map[string]string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		value = strings.TrimSpace(values[key])
	}
	if value == "" {
		value = fallback
	}
	return value
}

func envIntFrom(key string, fallback int, values map[string]string) int {
	value := envFrom(key, "", values)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBoolFrom(key string, fallback bool, values map[string]string) bool {
	value := strings.ToLower(envFrom(key, "", values))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

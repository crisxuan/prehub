package editorial

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/prehub/prehub/backend/internal/domain"
)

var (
	markdownLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	inlineCodePattern   = regexp.MustCompile("`([^`]+)`")
	htmlTagPattern      = regexp.MustCompile(`<[^>]+>`)
	spacePattern        = regexp.MustCompile(`\s+`)
)

func SummarizeReadme(readme string, fallback string) string {
	cleaned := strings.TrimSpace(readme)
	if cleaned == "" {
		return strings.TrimSpace(fallback)
	}

	lines := strings.Split(cleaned, "\n")
	parts := make([]string, 0, 4)
	inCodeBlock := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock || strings.Contains(line, "<!--") || shouldSkipReadmeLine(line) {
			continue
		}

		line = cleanMarkdownLine(line)
		if line == "" || shouldSkipReadmeLine(line) {
			continue
		}
		parts = append(parts, line)
		if runeLen(strings.Join(parts, " ")) >= 360 || len(parts) >= 4 {
			break
		}
	}

	summary := strings.TrimSpace(strings.Join(parts, " "))
	if summary == "" {
		return strings.TrimSpace(fallback)
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" && !sameOpening(summary, fallback) {
		summary = fallback + " " + summary
	}
	return truncateRunes(summary, 420)
}

func WriteRepositoryNarrative(repo domain.Repository) domain.Repository {
	if strings.TrimSpace(repo.Summary) == "" || looksBadSummary(repo.Summary) {
		repo.Summary = strings.TrimSpace(repo.Description)
	}
	repo.Reason = RecommendationReason(repo)
	repo.Caveat = RecommendationCaveat(repo)
	return repo
}

func RecommendationReason(repo domain.Repository) string {
	intro := firstSentence(repo.Summary)
	if intro == "" {
		intro = firstSentence(repo.Description)
	}
	if intro == "" {
		intro = repo.FullName + " 是一个仍需进一步核验 README 的开源项目"
	}

	problem := problemStatement(repo)
	proof := proofStatement(repo)
	reason := fmt.Sprintf("README 将它定位为「%s」，%s。%s，适合作为「%s」的候选项目。", normalizeIntro(intro), problem, proof, useCase(repo))
	return truncateRunes(reason, 300)
}

func RecommendationCaveat(repo domain.Repository) string {
	parts := []string{}
	license := strings.ToLower(strings.TrimSpace(repo.License))
	if license == "" || license == "unknown" || license == "other" {
		parts = append(parts, "license 信息不够明确，采用前要确认授权边界")
	} else if strings.Contains(license, "agpl") || strings.Contains(license, "gpl") {
		parts = append(parts, "license 约束较强，商业或闭源集成前要先评估合规")
	}

	if repo.Stars < 300 {
		parts = append(parts, "项目还偏早期，需要重点验证核心路径是否稳定")
	} else if repo.Stars < 1500 {
		parts = append(parts, "社区规模还在成长，建议先用小场景试跑")
	}

	if daysSincePush(repo.PushedAt) > 90 {
		parts = append(parts, "最近维护活跃度一般，需要检查 issue 和 release 节奏")
	}
	if strings.TrimSpace(repo.Summary) == "" || strings.TrimSpace(repo.Summary) == strings.TrimSpace(repo.Description) {
		parts = append(parts, "README 摘要信息有限，发布前建议再人工扫一遍文档")
	}

	parts = append(parts, "展示前建议跑通 README quickstart，并确认部署成本、外部依赖和数据安全边界")
	return truncateRunes(strings.Join(unique(parts), "；")+"。", 260)
}

func problemStatement(repo domain.Repository) string {
	topics := topicSet(repo.Topics)
	text := strings.ToLower(strings.Join([]string{repo.Name, repo.Description, repo.Summary, strings.Join(repo.Topics, " ")}, " "))
	has := func(values ...string) bool {
		for _, value := range values {
			if topics[value] || strings.Contains(text, value) {
				return true
			}
		}
		return false
	}

	switch {
	case has("image-generation", "image-editing", "text-to-image", "image-to-image", "diffusion", "stable-diffusion", "comfyui", "multimodal", "vision"):
		return "核心痛点是把图像生成、编辑或多模态能力接入真实创作工作流"
	case has("prompt", "prompts", "prompt-engineering", "prompt-management", "prompt-library", "prompt-template", "system-prompt"):
		return "核心痛点是把 prompt 技巧、模板和工作流沉淀成可复用资产"
	case has("skill", "skills", "claude-code", "codex", "agent-client-protocol", "tool-use"):
		return "核心痛点是让 AI agent 通过可复用 skills 和工具流程处理具体任务"
	case has("cost-optimization", "model-routing", "router"):
		return "核心痛点是多模型调用时的成本、准确率和路由决策"
	case has("evaluation", "evals", "tracing", "observability", "monitoring", "debugging", "guardrails"):
		return "核心痛点是 AI agent 从开发调试到评测、监控和持续优化的闭环"
	case has("mcp", "tool", "function-calling", "api-gateway", "gateway"):
		return "核心痛点是 AI agent 接入工具/API 时的统一入口、治理和可靠调用"
	case has("rag", "knowledge-base", "vector-search", "semantic-search", "wiki"):
		return "核心痛点是把业务文档和知识库变成可检索、可追问的 RAG 系统"
	case has("personal-assistant", "assistant", "privacy"):
		return "核心痛点是个人数据、工具编排和自动化任务的私有化管理"
	case has("codex", "claude-code", "copilot", "agent-client-protocol", "coding"):
		return "核心痛点是多种 AI coding agent 的本地编排、上下文和工具可视化"
	case has("workflow", "automation", "orchestrator"):
		return "核心痛点是把 AI 能力接入真实业务流程，而不是停留在聊天界面"
	case has("bi", "analytics", "data-analysis"):
		return "核心痛点是让 agent 参与数据分析、指标理解和业务决策"
	case has("medical-imaging", "biomedical", "computer-vision"):
		return "核心痛点是专业影像场景中机器学习方案的端到端研发"
	case has("infrastructure-as-code", "terraform", "gcp", "security"):
		return "核心痛点是把生成式 AI 基础设施用可复制、可审计的方式落地"
	default:
		return "核心价值是把 README 中描述的能力做成可以直接评估的开源实现"
	}
}

func proofStatement(repo domain.Repository) string {
	parts := []string{}
	if repo.Stars >= 100 && repo.Stars < 1500 {
		parts = append(parts, "它已经有基础社区关注但还没过度出圈")
	} else if repo.Stars < 8000 {
		parts = append(parts, "它有一定社区验证，同时仍保留发现潜力")
	} else {
		parts = append(parts, "它的社区验证度较高，但仍未进入超高 star 项目层级")
	}

	days := daysSincePush(repo.PushedAt)
	if days <= 14 {
		parts = append(parts, "最近两周仍在维护")
	} else if days <= 45 {
		parts = append(parts, "近期仍有更新")
	}

	if isClearLicense(repo.License) {
		parts = append(parts, "license 清晰")
	}
	if strings.TrimSpace(repo.Language) != "" {
		parts = append(parts, "主要技术栈是 "+repo.Language)
	}
	return strings.Join(unique(parts), "，")
}

func useCase(repo domain.Repository) string {
	text := strings.ToLower(strings.Join([]string{repo.Description, repo.Summary, strings.Join(repo.Topics, " ")}, " "))
	switch {
	case strings.Contains(text, "image generation") || strings.Contains(text, "image editing") || strings.Contains(text, "gpt-image") || strings.Contains(text, "comfyui") || strings.Contains(text, "diffusion") || strings.Contains(text, "multimodal"):
		return "AI 绘图和多模态工作流"
	case strings.Contains(text, "prompt engineering") || strings.Contains(text, "prompt library") || strings.Contains(text, "prompt template") || strings.Contains(text, "system prompt"):
		return "Prompt 技巧和模板沉淀"
	case strings.Contains(text, "codex skill") || strings.Contains(text, "claude code skill") || strings.Contains(text, "agent skill") || strings.Contains(text, "skills/"):
		return "AI Skills 和 agent 工作流"
	case strings.Contains(text, "model-routing") || strings.Contains(text, "cost-optimization"):
		return "LLM 成本优化和模型路由"
	case strings.Contains(text, "evaluation") || strings.Contains(text, "evals") || strings.Contains(text, "observability"):
		return "AI agent 评测与优化"
	case strings.Contains(text, "mcp") || strings.Contains(text, "agent"):
		return "AI agent 工具链"
	case strings.Contains(text, "rag") || strings.Contains(text, "knowledge"):
		return "知识库问答和检索增强"
	case strings.Contains(text, "workflow") || strings.Contains(text, "automation"):
		return "业务自动化"
	default:
		return "同类问题选型"
	}
}

func shouldSkipReadmeLine(line string) bool {
	if line == "" {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(line))
	return strings.HasPrefix(lower, "![") ||
		strings.HasPrefix(lower, "<img") ||
		strings.HasPrefix(lower, "<p") ||
		strings.HasPrefix(lower, "<div") ||
		strings.HasPrefix(lower, "<a") ||
		strings.HasPrefix(lower, "|") ||
		strings.HasPrefix(lower, "---") ||
		lower == "#" ||
		lower == "##" ||
		lower == "installation" ||
		lower == "install" ||
		lower == "usage" ||
		lower == "quickstart" ||
		lower == "quick start" ||
		lower == "getting started" ||
		lower == "features" ||
		lower == "license" ||
		lower == "documentation" ||
		lower == "table of contents" ||
		lower == "contents" ||
		strings.Contains(lower, "marketing notes") ||
		strings.Contains(lower, "image assets") ||
		strings.Contains(lower, "option 1:") ||
		strings.Contains(lower, "cloud-hosted") ||
		strings.Contains(lower, "choose your solution") ||
		strings.Contains(lower, "star history") ||
		strings.Contains(lower, "join discord") ||
		strings.Contains(lower, "shields.io") ||
		strings.Contains(lower, "badge")
}

func looksBadSummary(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lower, "<!--") ||
		strings.Contains(lower, "</picture>") ||
		strings.Contains(lower, "marketing notes") ||
		strings.Contains(lower, "image assets") ||
		strings.Contains(lower, "option 1: cloud-hosted")
}

func cleanMarkdownLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "#>*-0123456789. ")
	line = markdownLinkPattern.ReplaceAllString(line, "$1")
	line = inlineCodePattern.ReplaceAllString(line, "$1")
	line = htmlTagPattern.ReplaceAllString(line, " ")
	line = strings.ReplaceAll(line, "**", "")
	line = strings.ReplaceAll(line, "__", "")
	line = strings.ReplaceAll(line, "<br>", " ")
	line = strings.ReplaceAll(line, "<br/>", " ")
	line = strings.ReplaceAll(line, "&amp;", "&")
	return strings.TrimSpace(spacePattern.ReplaceAllString(line, " "))
}

func firstSentence(value string) string {
	value = cleanMarkdownLine(value)
	if value == "" {
		return ""
	}
	splitters := []string{"。", ". ", "！", "! ", "？", "? "}
	end := len(value)
	for _, splitter := range splitters {
		if index := strings.Index(value, splitter); index > 0 && index < end {
			end = index
		}
	}
	return strings.TrimSpace(strings.TrimRight(value[:end], ".。!！?？"))
}

func sameOpening(a string, b string) bool {
	firstA := strings.ToLower(firstSentence(a))
	firstB := strings.ToLower(firstSentence(b))
	if firstA == "" || firstB == "" {
		return false
	}
	return strings.Contains(firstA, firstB) || strings.Contains(firstB, firstA)
}

func normalizeIntro(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	for _, prefix := range []string{"this project is ", "this project ", "this is "} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(value[len(prefix):])
		}
	}
	return strings.TrimSpace(value)
}

func topicSet(topics []string) map[string]bool {
	set := map[string]bool{}
	for _, topic := range topics {
		set[strings.ToLower(strings.TrimSpace(topic))] = true
	}
	return set
}

func isClearLicense(license string) bool {
	switch strings.ToLower(strings.TrimSpace(license)) {
	case "mit", "apache-2.0", "bsd-2-clause", "bsd-3-clause", "isc", "mpl-2.0":
		return true
	default:
		return false
	}
}

func daysSincePush(pushedAt string) float64 {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(pushedAt))
	if err != nil || parsed.IsZero() {
		return 9999
	}
	return time.Since(parsed).Hours() / 24
}

func unique(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func truncateRunes(value string, max int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "..."
}

func runeLen(value string) int {
	return len([]rune(value))
}

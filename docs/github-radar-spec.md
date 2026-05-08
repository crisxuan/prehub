# PreHub GitHub Radar 大看板规格说明

最后更新：2026-05-08

状态：Draft v1，后续开发以本文为准。

## 1. 背景与定位

PreHub 当前已经具备 GitHub 项目采集、候选评分、每日推荐、分类推荐、项目详情和后台审核能力。下一阶段目标是从“每天推荐一个项目”升级为“GitHub 项目雷达”：持续监控 GitHub 项目的变化，识别近期飙升项目，展示 star 曲线和活动事件，并解释项目为什么值得关注。

一句话定位：

```text
PreHub Radar 是一个面向开发者的 GitHub 趋势大看板：发现正在变热、解决真实痛点、仍有潜力的开源项目。
```

## 2. 产品目标

- 发现近期增长明显的 GitHub 项目，而不只是展示高 star 项目。
- 实时或近实时监控重点项目的 stars、forks、issues、push、release 等变化。
- 绘制 star 曲线，支持 1h、24h、7d、30d、90d 观察窗口。
- 在 AI、AI 绘图、多模态、Prompt、AI Skills、开发工具等分类下展示飙升榜。
- 解释项目为什么上涨、解决什么痛点、是否有潜力、是否有风险。
- 为每日推荐提供更好的候选来源：从“人工/搜索发现”升级为“趋势发现 + 质量过滤 + 编辑审核”。

## 3. 非目标

- 不做 GitHub 的完整替代品。
- 不全量镜像 GitHub。
- 不克隆仓库源码。
- 不对所有 GitHub 项目做分钟级监控。
- Radar 页面中的“已监控仓库”是 PreHub 当前分类采样池口径，不代表 GitHub 全量仓库数量。
- 不把 star 数当作唯一推荐依据。
- 不默认抓取 GitHub 网页 HTML；优先使用 GitHub 官方 API 和公开事件数据。
- MVP 不做复杂社媒传播归因，例如 X、Reddit、Hacker News、微信公众号等外部传播分析。

## 4. 核心用户故事

- 作为开发者，我希望看到今天、过去 24 小时、过去 7 天哪些项目增长最快。
- 作为 AI 工程师，我希望单独查看 AI、Agent、Prompt、绘图、多模态、Skills 等方向的热门项目。
- 作为独立开发者，我希望发现 star 不高但增长很快、解决明确痛点的潜力项目。
- 作为编辑，我希望从趋势榜中挑选每日推荐候选，并看到推荐理由和风险提示。
- 作为管理员，我希望把项目加入监控列表，并设置刷新频率。
- 作为用户，我希望进入项目详情页后看到 star 曲线、近期事件、README 摘要和 GitHub 外链。

## 5. 信息架构

### 5.1 Public Web

新增或升级路由：

```text
/radar
/radar?category=ai&window=24h
/radar/projects/{owner}/{repo}
/radar/watchlist
/r/{owner}/{repo}
```

说明：

- `/radar` 是趋势大看板主入口。
- `/r/{owner}/{repo}` 继续作为项目详情页，可逐步增强成 Radar 项目页。
- 首页可以保留每日推荐，但应增加 Radar 入口和今日飙升模块。

### 5.2 Admin Console

新增后台路由：

```text
/admin/radar
/admin/radar/watchlist
/admin/radar/jobs
/admin/radar/events
/admin/radar/trends
```

说明：

- `/admin/radar/watchlist` 管理需要高频监控的项目。
- `/admin/radar/jobs` 查看采集任务、失败记录、限流状态。
- `/admin/radar/trends` 查看趋势分计算结果，支持推送到候选队列。

## 6. MVP 页面规格

### 6.1 Radar 首页

页面顶部：

- 已监控仓库数：仅统计当前分类已加入 Radar watchlist / 采样池的仓库。
- 过去 1 小时新增 stars。
- 过去 24 小时新增 stars。
- 今日新增候选项目数。
- GitHub API 剩余额度和采集健康状态。
- 必须显示口径说明：当前是 PreHub 的 Radar 采样池，从本地候选库和后台 watchlist 中挑选仓库持续采样，不代表 GitHub 全量仓库。

主内容区：

- 飙升榜：
  - 1h star delta
  - 24h star delta
  - 7d star delta
  - acceleration score
- 潜力榜：
  - stars 在 10-12000 之间。
  - 24h/7d 增长明显。
  - 排除 awesome list、spam、凭据仓库、长期不维护仓库。
- 分类榜：
  - 全部：跨分类聚合所有已监控仓库，仓库按 repository_id 去重。
  - AI
  - AI 绘图/多模态
  - Prompt 技巧
  - AI Skills/工作流
  - Web 前端
  - 开发工具
  - 数据与数据库
  - 后端基础设施
- 实时事件流：
  - StarEvent / WatchEvent
  - ForkEvent
  - PushEvent
  - ReleaseEvent
  - IssuesEvent
  - PullRequestEvent

项目卡片字段：

- 项目图标。
- `owner/name`。
- GitHub 外链。
- description。
- language、license、topics。
- stars 当前值。
- 1h / 24h / 7d star delta。
- trend score。
- 最近一次 push/release 时间。
- 推荐解释摘要。

### 6.2 项目详情页

顶部项目信息：

- 项目图标。
- owner/name。
- GitHub 外链。
- stars、forks、open issues。
- language、license、topics。
- README 摘要。
- 推荐理由和风险提醒。

趋势区：

- star 曲线：
  - 24h
  - 7d
  - 30d
  - 90d
- 关键指标：
  - `stars_total`
  - `stars_delta_1h`
  - `stars_delta_24h`
  - `stars_delta_7d`
  - `forks_delta_7d`
  - `activity_events_24h`
  - `last_pushed_at`
  - `last_released_at`

事件区：

- 最近 push。
- 最近 release。
- issue/PR 活动。
- fork/star 事件聚合。

解释区：

- 最近为什么增长。
- 解决什么痛点。
- 谁适合关注。
- 是否存在风险：
  - license 不清楚。
  - README 不完整。
  - 短期 star 异常。
  - 长期无维护。
  - 疑似营销仓库或 awesome list。

### 6.3 Admin Watchlist

功能：

- 添加 `{owner}/{repo}` 到监控列表。
- 设置监控等级：
  - `hot`：5-10 分钟刷新。
  - `watch`：30 分钟刷新。
  - `candidate`：6 小时刷新。
  - `archive`：24 小时刷新。
- 设置分类。
- 手动触发刷新。
- 手动触发 star 历史回填。
- 查看最近采集失败原因。

## 7. 数据源策略

### 7.0 全网发现原则

Radar 不能只依赖手工 watchlist。正确架构是“两层池”：

- 全网发现池：持续从 GitHub Search、Public Events / GH Archive、主题 query、近期创建和近期更新窗口召回项目。
- 重点监控池：只把通过质量过滤、增长过滤或人工加入的项目放入 `monitored_repositories` 做高频采样。

原因：

- GitHub 没有“全量按趋势导出所有仓库”的单一 API。
- Search API 单次查询存在结果窗口限制，因此必须用 category、topic、keyword、created/pushed 时间窗、stars 区间做 query grid。
- GitHub Events / GH Archive 更适合发现全网新信号，再把候选项目升级进 Radar watchlist。
- 页面上的“已监控仓库”不是全网仓库数，而是当前进入重点监控池的仓库数；真正的全网覆盖能力看“全网发现池召回量、候选库量、每小时新项目数、升入监控池数量”。

MVP 实现要求：

- Worker 默认按多个分类运行 discovery query grid，不再只跑单一 `PREHUB_INITIAL_CATEGORY`。
- 每个 query 支持分页，默认 `per_page=30`、`pages=2`，有 GitHub token 时可提高。
- 发现后先做基础质量评分，合格项目写入 `repository_candidates`，并自动加入对应分类的 Radar candidate tier。
- 对预算内的高质量项目抓 README 摘要和图标；对少量项目抓 stargazers `starred_at` 回填短期 star 曲线。
- 通过环境变量控制预算：`PREHUB_DISCOVERY_CATEGORIES`、`PREHUB_DISCOVERY_PER_PAGE`、`PREHUB_DISCOVERY_PAGES`、`PREHUB_DISCOVERY_MAX_SEARCHES`、`PREHUB_DISCOVERY_README_LIMIT`、`PREHUB_DISCOVERY_STARGAZER_REPOS`、`PREHUB_RADAR_SEED_CATEGORIES`。

### 7.1 GitHub REST API

优先使用 REST API 获取仓库基础信息、README、topics、languages、stargazers 和 events。

核心 endpoint：

| 需求 | Endpoint | 用途 |
| --- | --- | --- |
| 仓库元数据 | `GET /repos/{owner}/{repo}` | stars、forks、issues、license、pushed_at、default_branch。 |
| README | `GET /repos/{owner}/{repo}/readme` | 摘要、项目定位、图标提取、质量判断。 |
| Topics | `GET /repos/{owner}/{repo}/topics` | 分类、召回、过滤。 |
| Languages | `GET /repos/{owner}/{repo}/languages` | 语言分布。 |
| Stargazers | `GET /repos/{owner}/{repo}/stargazers` | 使用 star media type 获取 `starred_at`。 |
| Repo Events | `GET /repos/{owner}/{repo}/events` | 最近公开事件。 |
| Public Events | `GET /events` | 全局公开事件抽样发现。 |
| Rate Limit | `GET /rate_limit` | 配额监控，运行中优先读响应头。 |

### 7.2 GitHub GraphQL API

GraphQL 用于后续批量取多个仓库字段，MVP 不强依赖。适合：

- 一次查询多个 repository。
- 拉取 repository connections。
- 精准控制字段。
- 降低 REST 多 endpoint 请求次数。

### 7.3 GH Archive

GH Archive / ClickHouse 公共事件流作为 Radar 的外部趋势来源，用于补齐本地快照积累不足时的 7d / 30d star 增量与事件热度。

用途：

- 扫描全站 WatchEvent / ForkEvent / PushEvent。
- 找到不在本地 watchlist 的新兴项目。
- 做全局趋势候选召回。
- 对本地已监控仓库按窗口回填 `repository_external_trends` 与 bucket 曲线。
- Radar 排行与项目曲线优先使用外部趋势数据；没有外部数据时才回退到本地 snapshots / star events。

### 7.4 Webhook

Webhook 只适合用户或组织授权后监控自己拥有或安装过 GitHub App 的仓库。MVP 不做。

## 8. 采集等级与刷新频率

| Tier | 名称 | 用途 | 刷新频率 |
| --- | --- | --- | --- |
| 0 | `hot` | 今日推荐、热点项目、人工重点关注 | 5-10 分钟 |
| 1 | `watch` | 用户/管理员加入 watchlist 的项目 | 30 分钟 |
| 2 | `candidate` | 候选队列、分类榜项目 | 6 小时 |
| 3 | `archive` | 已入库但不活跃项目 | 24 小时 |

刷新原则：

- 每个仓库刷新任务必须去重。
- archived、disabled、长期无更新项目自动降级。
- star 增速明显项目自动升级。
- GitHub API 配额低时优先刷新 Tier 0 和 Tier 1。
- 所有请求记录 `x-ratelimit-*` 响应头。
- 使用 ETag/Last-Modified 做条件请求。
- 遇到 secondary rate limit 时进入退避，不继续并发打 API。

## 9. 数据模型

保留现有 `repositories` 表，新增以下表。

### 9.1 monitored_repositories

```sql
CREATE TABLE monitored_repositories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  category text NOT NULL DEFAULT 'ai',
  tier text NOT NULL DEFAULT 'candidate',
  status text NOT NULL DEFAULT 'active',
  refresh_interval_seconds integer NOT NULL DEFAULT 21600,
  last_scheduled_at timestamptz,
  last_refreshed_at timestamptz,
  next_refresh_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (repository_id, category)
);
```

### 9.2 repository_metric_snapshots

```sql
CREATE TABLE repository_metric_snapshots (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  captured_at timestamptz NOT NULL DEFAULT now(),
  stars_count integer NOT NULL DEFAULT 0,
  forks_count integer NOT NULL DEFAULT 0,
  watchers_count integer NOT NULL DEFAULT 0,
  open_issues_count integer NOT NULL DEFAULT 0,
  subscribers_count integer NOT NULL DEFAULT 0,
  pushed_at timestamptz,
  source text NOT NULL DEFAULT 'github_rest',
  UNIQUE (repository_id, captured_at)
);
```

索引：

```sql
CREATE INDEX repository_metric_snapshots_repo_time_idx
  ON repository_metric_snapshots (repository_id, captured_at DESC);
```

### 9.3 repository_activity_events

```sql
CREATE TABLE repository_activity_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  repository_id uuid REFERENCES repositories(id) ON DELETE CASCADE,
  github_event_id text NOT NULL UNIQUE,
  event_type text NOT NULL,
  actor_login text,
  actor_avatar_url text,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL,
  ingested_at timestamptz NOT NULL DEFAULT now()
);
```

### 9.4 repository_star_events

只对重点项目回填，不能全量滥用。

```sql
CREATE TABLE repository_star_events (
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  github_user_id bigint NOT NULL,
  login text NOT NULL,
  starred_at timestamptz NOT NULL,
  ingested_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (repository_id, github_user_id)
);
```

### 9.5 repository_trend_scores

```sql
CREATE TABLE repository_trend_scores (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  repository_id uuid NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
  category text NOT NULL DEFAULT 'ai',
  trend_window text NOT NULL,
  star_delta integer NOT NULL DEFAULT 0,
  fork_delta integer NOT NULL DEFAULT 0,
  issue_delta integer NOT NULL DEFAULT 0,
  activity_events integer NOT NULL DEFAULT 0,
  velocity_score numeric(8, 3) NOT NULL DEFAULT 0,
  acceleration_score numeric(8, 3) NOT NULL DEFAULT 0,
  novelty_score numeric(8, 3) NOT NULL DEFAULT 0,
  quality_score numeric(8, 3) NOT NULL DEFAULT 0,
  suspicious_score numeric(8, 3) NOT NULL DEFAULT 0,
  trend_score numeric(8, 3) NOT NULL DEFAULT 0,
  explanation text NOT NULL DEFAULT '',
  calculated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (repository_id, category, window, calculated_at)
);
```

推荐查询索引：

```sql
CREATE INDEX repository_trend_scores_rank_idx
  ON repository_trend_scores (category, trend_window, trend_score DESC, calculated_at DESC);
```

## 10. 趋势计算

### 10.1 窗口

必须支持：

```text
1h
24h
7d
30d
90d
```

MVP 至少支持：

```text
24h
7d
```

### 10.2 基础指标

```text
star_delta_window = current_stars - stars_at_window_start
fork_delta_window = current_forks - forks_at_window_start
issue_delta_window = current_open_issues - issues_at_window_start
velocity = star_delta_window / window_hours
acceleration = velocity_recent / max(velocity_baseline, 1)
```

如果窗口起点没有精确 snapshot，使用窗口起点前后最近两点做近似。MVP 可先用“窗口起点前最近 snapshot”。

### 10.3 Trend Score

MVP 计算公式：

```text
trend_score =
  star_delta_score * 0.35
+ velocity_score * 0.20
+ acceleration_score * 0.15
+ activity_score * 0.10
+ quality_score * 0.10
+ novelty_score * 0.10
- suspicious_score
```

说明：

- `star_delta_score`：窗口内 star 增量归一化。
- `velocity_score`：单位时间增长速度。
- `acceleration_score`：最近增长是否显著高于过去基线。
- `activity_score`：push、release、issue、PR 活跃度。
- `quality_score`：复用现有质量评分。
- `novelty_score`：偏向未过度出圈但增长明确的项目。
- `suspicious_score`：异常增长、低文档质量、spam 风险、长期无维护等扣分。

### 10.4 潜力项目规则

潜力榜必须区别于“高 star 榜”。

默认规则：

```text
stars_count BETWEEN 10 AND 12000
archived = false
disabled = false
pushed_at > now() - 180 days
not awesome-list
not spam/piracy/credential-bypass/free-api-key
trend_score >= threshold
```

AI 类项目额外关注：

- Agent 工具链。
- RAG / 数据连接。
- Prompt workflow。
- AI image / multimodal workflow。
- Skills / MCP / coding agent workflow。
- 模型路由、评测、推理、部署。

## 11. Star 曲线

### 11.1 默认曲线

所有被监控项目都从接入时开始保存 `repository_metric_snapshots`。star 曲线优先使用 snapshots。

优点：

- 成本低。
- API 压力小。
- 适合长期监控。

缺点：

- 无法还原接入前的完整历史。

### 11.2 历史回填

对重点项目可以调用 stargazers API，使用 GitHub star media type 获取 `starred_at`，回填 `repository_star_events`。

限制：

- 只对 Tier 0 / Tier 1 或人工触发项目执行。
- 大型项目需要分页、断点续传、限流。
- 回填任务必须可暂停、可恢复。
- 默认不存不必要的用户信息，只存公开 login、github_user_id、starred_at。

### 11.3 曲线展示规则

- 1h / 24h / 7d / 30d 优先使用 `repository_external_trends` 的外部事件流回填数据。
- 没有外部数据时，1h / 24h / 7d 使用 snapshots；30d / 90d 优先使用 snapshots，如果有 stargazers backfill，则补充历史。
- 数据不足时展示空状态：`数据正在积累，首次接入后需要至少两个采样点生成曲线。`
- Radar 首页右侧展示“项目曲线预览”，默认选中榜首项目；点击榜单行只切换右侧曲线，不进入详情页。
- 项目名称链接进入 `/r/{owner}/{repo}`；GitHub 按钮打开仓库外链，二者都不应触发行选择。

## 12. 后台任务

新增 worker job：

```text
radar.schedule_refresh
radar.refresh_repository_metrics
radar.ingest_repository_events
radar.calculate_trend_scores
radar.backfill_star_history
radar.promote_trending_candidates
radar.cleanup_old_events
```

### 12.1 radar.refresh_repository_metrics

输入：

```json
{
  "repositoryId": "...",
  "owner": "snyk",
  "repo": "agent-scan",
  "priority": "hot"
}
```

处理：

1. 调 GitHub repo metadata。
2. upsert `repositories`。
3. insert `repository_metric_snapshots`。
4. 更新 `monitored_repositories.last_refreshed_at`。
5. 根据趋势调整 tier 和 next_refresh_at。

### 12.2 radar.calculate_trend_scores

处理：

1. 对每个 category/window 查询当前 snapshot 和窗口起点 snapshot。
2. 计算 delta、velocity、acceleration。
3. 合并 quality score。
4. 计算 suspicious score。
5. 写入 `repository_trend_scores`。

### 12.3 radar.promote_trending_candidates

处理：

1. 读取 trend_score 高的项目。
2. 排除已发布、黑名单、低质量项目。
3. 写入 `repository_candidates`。
4. 标记 source 为 `radar_trending`。

## 13. API 规格

Go API 新增 v1 endpoints，Next.js BFF 做同名代理。

### 13.1 GET /v1/radar/overview

Query：

```text
category=ai
window=24h
```

`category` 支持具体分类 slug，也支持 `all`。`all` 表示跨所有分类聚合 Radar watchlist / 采样池仓库；统计仓库数、候选项目数和趋势榜时必须按 `repository_id` 去重，避免同一仓库被多个分类重复计数。

Response：

```json
{
  "category": "ai",
  "window": "24h",
  "monitoredCount": 328,
  "starDelta": 4182,
  "candidateCount": 42,
  "apiHealth": {
    "status": "ok",
    "remaining": 4120,
    "resetAt": "2026-05-08T16:00:00Z"
  },
  "topTrending": [],
  "topPotential": [],
  "recentEvents": []
}
```

### 13.2 GET /v1/radar/trending

Query：

```text
category=ai
window=24h
limit=50
sort=trend_score
```

Response item：

```json
{
  "repository": {},
  "window": "24h",
  "starDelta": 128,
  "forkDelta": 12,
  "activityEvents": 19,
  "velocityScore": 84.2,
  "accelerationScore": 62.1,
  "trendScore": 88.4,
  "explanation": "过去 24 小时 star 增长明显，同时最近有 release 和持续 push，适合进入候选队列。"
}
```

### 13.3 GET /v1/radar/repositories/{owner}/{repo}/metrics

Query：

```text
window=7d
interval=1h
```

Response：

```json
{
  "repository": {},
  "window": "7d",
  "points": [
    {
      "capturedAt": "2026-05-08T10:00:00Z",
      "stars": 2100,
      "forks": 120,
      "openIssues": 18
    }
  ],
  "summary": {
    "starDelta": 320,
    "forkDelta": 18,
    "activityEvents": 44
  }
}
```

### 13.4 GET /v1/radar/events

Query：

```text
category=ai
repository=owner/name
eventType=ReleaseEvent
limit=50
```

### 13.5 POST /v1/admin/radar/watchlist

Body：

```json
{
  "url": "https://github.com/snyk/agent-scan",
  "category": "ai",
  "tier": "watch"
}
```

### 13.6 POST /v1/admin/radar/backfill

用途：从 GH Archive / ClickHouse 事件流回填当前 Radar 采样池的 1h / 24h / 7d / 30d 趋势数据。

Body：

```json
{
  "category": "ai",
  "windows": ["1h", "24h", "7d", "30d"],
  "limit": 1000,
  "batchSize": 250
}
```

Response：

```json
{
  "status": "accepted",
  "source": "clickhouse_gharchive",
  "category": "ai",
  "results": [
    {
      "window": "7d",
      "repositoryCount": 1000,
      "matchedCount": 960,
      "starDelta": 5200,
      "activityEvents": 88000,
      "windowStartedAt": "2026-05-01T00:00:00Z",
      "windowEndedAt": "2026-05-08T00:00:00Z"
    }
  ]
}
```

### 13.7 POST /v1/admin/radar/repositories/{owner}/{repo}/backfill-stars

Body：

```json
{
  "maxPages": 10
}
```

## 14. 前端设计要求

### 14.1 Dashboard

视觉方向延续当前 PreHub 绿色、白底、轻边框、数据卡片风格，但信息密度更高。

组件：

- KPI strip。
- 分类 tabs。
- 分类 tabs 必须包含 `全部`，用于跨分类查看趋势；后台发布、加入监控等写入动作仍必须选择具体分类。
- window segmented control：`1h / 24h / 7d / 30d`。
- 排行榜 table/list。
- 排行榜行点击只更新右侧项目曲线预览。
- star delta badge。
- mini sparkline。
- 事件流。
- 数据健康状态。

### 14.2 Charts

推荐先使用 Recharts 或 ECharts。

MVP 图表：

- line chart：star 曲线。
- area chart：star 增长趋势。
- bar chart：24h/7d 增量对比。
- sparkline：榜单卡片内小曲线。

### 14.3 空状态

必须有：

- 没有监控项目。
- 数据仍在采样中。
- GitHub API 限流。
- 项目无足够历史点。
- 当前分类没有趋势项目。

## 15. AI 解释生成

AI 解释不是趋势排序的唯一依据，而是趋势排序之后的说明层。

输入：

- README summary。
- description。
- topics。
- language。
- license。
- star delta。
- recent events。
- quality score。
- caveat。

输出：

```json
{
  "painPoint": "它解决什么问题",
  "whyTrending": "最近为什么可能在增长",
  "whoShouldWatch": "适合哪些人关注",
  "risk": "使用前注意什么",
  "recommendationReason": "是否适合作为每日推荐候选"
}
```

要求：

- 不编造外部传播原因。
- 如果没有证据，只能说“从 GitHub 内部活动看”。
- 明确区分事实和推断。
- 高风险项目不得自动进入每日推荐。

## 16. 安全、合规与风控

- GitHub token 只存在服务端。
- API 响应中不得暴露 token、raw headers 中的敏感信息。
- 采集任务必须限流。
- suspicious score 必须参与排序。
- 对疑似刷星项目降权。
- 对 malware、piracy、credential bypass、free api key、jailbreak 等风险项目降权或过滤。
- star event 中只存公开必要字段。

## 17. 分阶段开发计划

### Phase 0：规格与数据结构

- 新增本 spec。
- 新增 DB migration：
  - `monitored_repositories`
  - `repository_metric_snapshots`
  - `repository_activity_events`
  - `repository_trend_scores`
- 更新 OpenAPI。
- 更新 Go domain types。

验收：

- migration 可在空库执行。
- Go 编译通过。

### Phase 1：Metric Snapshot MVP

- Admin 可添加 watchlist 项目。
- Worker 可刷新仓库元数据并写入 snapshots。
- API 可返回项目 metrics points。
- 项目详情页展示 star 曲线。

验收：

- 至少 20 个项目能产生 2 个以上 snapshot。
- 项目详情页能看到 star 曲线。

### Phase 2：Trend Score 与 Radar 首页

- 实现 24h / 7d trend score。
- `/radar` 展示分类趋势榜。
- 首页增加今日飙升模块。

验收：

- 能按 category/window 获取 trend list。
- 榜单包含 star delta、trend score、推荐解释。

### Phase 3：Star History Backfill

- 对单个项目手动触发 stargazers 历史回填。
- 支持断点续传和 max pages。
- 项目详情页可展示更长时间 star 曲线。

验收：

- 对一个中小型仓库能成功回填 star events。
- 不触发 GitHub secondary rate limit。

### Phase 4：Events 与全局发现

- 接入 repo events。
- 接入 public events 或 GH Archive；MVP 已先通过 ClickHouse GH Archive 对采样池做 7d / 30d 外部趋势回填。
- Radar 展示事件流。
- 从事件中发现新候选项目。

验收：

- 事件去重入库。
- 事件流能按 category/repo/event_type 过滤。

### Phase 5：趋势解释与推荐闭环

- AI 生成趋势解释。
- 高分趋势项目自动进入候选队列。
- Admin 可以一键发布为每日推荐。

验收：

- 趋势项目可进入 `repository_candidates`。
- 推荐理由包含 README、趋势、风险三类信息。

## 18. MVP 验收标准

Radar MVP 完成必须满足：

- 本地 Docker Compose 可启动完整系统。
- 至少支持 200 个 monitored repositories。
- 至少支持 24h 和 7d 两个趋势窗口。
- 每个 monitored repository 至少保存 stars/forks/issues snapshots。
- `/radar` 能展示分类趋势榜。
- 项目详情页能展示 star 曲线。
- Admin 能添加/移除 watchlist 项目。
- Worker 有基本限流、失败记录、去重和退避。
- API 有空状态和错误状态。
- 趋势榜不只按 stars 总数排序。
- AI 分类下优先展示有痛点、有增长、有潜力的项目。

## 19. 后续扩展

- 用户自定义 watchlist。
- 邮件/Slack/飞书趋势提醒。
- RSS：今日飙升、分类飙升、AI 飙升。
- 项目对比：两个项目 star 曲线和活跃度对比。
- Owner/Organization 看板。
- 外部传播归因：Hacker News、Reddit、X、Product Hunt。
- TimescaleDB 或分区表优化长期时间序列。

## 20. 参考资料

- [GitHub REST API rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)
- [GitHub GraphQL rate limits](https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api)
- [GitHub Events API](https://docs.github.com/en/rest/activity/events)
- [GitHub Stargazers API](https://docs.github.com/en/rest/activity/starring)
- [GH Archive](https://www.gharchive.org/)

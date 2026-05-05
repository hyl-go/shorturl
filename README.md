# Short-URL 

本文档基于当前仓库代码与配置梳理：目录结构、技术栈、数据存储、HTTP API、核心业务流程、实现要点，以及项目亮点与难点。

---

## 1. 项目概述

本项目是一个**短链接服务**：用户提交长 URL，系统生成短路径；访问短链时 **HTTP 302 重定向** 至目标地址；可选 **AI**（当前为 DeepSeek）对链接做分类、安全与元信息增强；后台提供 **统计、分析、AI 报告（异步）**、**链接 CRUD** 与 **运维性能面板**。

- **后端**：`shorturl/` — Go + [go-zero](https://github.com/zeromicro/go-zero) REST 服务，单进程内同时启动 **HTTP API** 与 **Asynq Worker/Scheduler**（访问日志、小时聚合、AI 报告、可选 GC）。
- **前端**：`shortUrlfrontend/` — Vue 3 + Vite + TypeScript + Element Plus + Pinia + ECharts，开发时通过 Vite 将 `/api` 代理到后端 `8888` 端口。

---

## 2. 仓库目录结构（要点）

```
short-url/
├── docker-compose.yaml          # MySQL 8.4、Redis（示例端口 13307 / 16379）
├── README.md
├── 技术文档.md                   # 本文件
├── shorturl/                     # Go 后端（go-zero）
│   ├── etc/shorturl-api.yaml     # 主配置（DSN、Redis、AI、Asynq、SSRF、限流等）
│   ├── shorturl.api              # API 类型与服务定义（goctl 源）
│   ├── shorturl.go               # main：REST Server + Asynq Worker + 定时任务
│   ├── short_url_map.sql         # short_url_map 表结构
│   ├── sequence.sql              # sequence 发号表（MySQL 模式/Redis 引导用）
│   ├── migration.sql             # 迁移：扩展字段、access_log、access_stats
│   ├── internal/
│   │   ├── config/               # 配置结构体
│   │   ├── handler/              # HTTP 处理器（路由入口）
│   │   ├── logic/                # 业务逻辑层
│   │   ├── svc/servicecontext.go # 依赖注入（DB、Redis、Sequence、Filter、AI、Worker…）
│   │   ├── ai/                   # AI Provider / DeepSeek / Fallback
│   │   ├── aireport/             # AI 报告异步任务、Redis 任务状态存储
│   │   ├── crawler/              # 页面拉取（受 SSRF 策略约束）
│   │   └── worker/               # Asynq：访问日志、统计聚合、链接 GC、AI 报告
│   ├── model/                    # short_url_map、sequence 的 CRUD + 缓存键
│   ├── pkg/                      # base62、md5、urlTool、ssrf、geoip、surlfilter、connect…
│   └── sequence/                 # 发号器：Redis INCR 或 MySQL 兼容
└── shortUrlfrontend/             # Vue 前端
    ├── vite.config.ts            # /api → 后端代理
    ├── src/
    │   ├── api/shorturl.ts        # Axios 封装与类型
    │   ├── config/admin.ts       # 管理端路由常量、本地登录标记（非后端鉴权）
    │   ├── router/index.ts       # 路由与 requiresAdmin 守卫
    │   ├── store/link.ts         # Pinia（链接列表等）
    │   └── views/                # Convert / LinkList / Analytics / Performance / AdminLogin
    └── package.json
```

---

## 3. 技术栈一览

| 层级 | 技术 | 在项目中的作用 |
|------|------|----------------|
| 后端语言/框架 | Go 1.24、go-zero REST | 路由、`httpx` 参数绑定、`sqlx` 访问 MySQL、中间件（限流、管理端 Token、Recover 等） |
| 数据库 | MySQL 8.x（InnoDB） | 短链映射表、访问日志与按小时汇总表、发号表 `sequence`（Redis 模式时用于 bootstrap） |
| 缓存与中间状态 | Redis | go-zero **CachedConn** 行缓存（`cache:shorturl:shortUrlMap:*`）；**INCR 发号**；**Cuckoo/Bloom** 近似存在性过滤；**HyperLogLog** 辅助 UV（`stats:uv:…`）；**Asynq** 队列与 **AI 报告任务状态** |
| 异步任务 | [Asynq](https://github.com/hibiken/asynq) + Cron Scheduler | 访问日志异步入库、按小时 `access_stats` 聚合、AI 分析报告任务、可选软删物理清理 |
| AI | DeepSeek Chat API（可配置）+ Fallback | 分类、安全等级、页面标题/描述、短链名建议；报告正文可异步生成 |
| 系统指标 | gopsutil | `/admin/performance` 采集主机 CPU/内存/磁盘等 |
| 前端 | Vue 3、Vite 8、TypeScript、Element Plus、Pinia、ECharts | 访客转链页、管理端列表/统计/性能面板；`marked`/`html2canvas`/`jspdf` 等用于展示与导出场景 |

---

## 4. MySQL 表结构与设计要点

### 4.1 `short_url_map`（核心映射表）

来源：`short_url_map.sql`，并经由 `migration.sql` 增加扩展列。

| 字段 | 说明 |
|------|------|
| `id` | 自增主键 |
| `create_at` / `update_at` | 创建与更新时间 |
| `create_by` | 创建者（预留） |
| `is_del` | 软删标记；配合可选 GC 物理清理 |
| `lurl` | 长链接；**唯一约束**（同长链不能重复插入新行） |
| `md5` | 长链 MD5，便于查询与去重 |
| `surl` | 短链路径段；**唯一约束** |
| `expire_at` | 过期时间，可为 NULL（长期有效） |
| `category` | 分类（如新闻/技术/购物…或「其他」） |
| `safety_status` | 安全：0 安全 / 1 可疑 / 2 危险（AI 或规则） |
| `page_title` / `page_description` | 爬取的页面元信息 |
| `ai_suggestions` | JSON：AI 建议的短链别名列表 |

**业务含义**：**短码与长链的真源（source of truth）**；布隆/布谷过滤器仅作加速与防穿透，以 DB 为准。

### 4.2 `sequence`（发号表）

来源：`sequence.sql`。MyISAM，`stub` 唯一，用于**历史 MySQL 发号**；当 **Sequence.Provider=redis** 时，Redis 计数器可通过 `BootstrapFromMysql` 读取 `MAX(id)` 对齐，避免与旧短码冲突。

### 4.3 `access_log`（访问明细）

`migration.sql` 定义：每次有效跳转（经 Show 逻辑）异步写入；含 `surl`、`access_time`、`ip`、地理、`user_agent`、解析后的 `device_type`/`os`/`browser`、`referer` 等。

**业务含义**：**统计接口的权威数据源**（见 `StatsLogic` 注释：`access_stats` 不直接参与在线查询，避免异步汇总延迟导致口径不一致）。

### 4.4 `access_stats`（按小时汇总）

`surl` + `date` + `hour` 唯一；由 **StatsWorker** 每小时从 `access_log` 聚合写入，用于离线分析或降载扩展，**与对外 Stats API 解耦**。

---

## 5. Redis 在项目中的职责

| 用途 | 键/机制 | 说明 |
|------|---------|------|
| ORM 行缓存 | `cache:shorturl:shortUrlMap:{id\|lurl\|md5\|surl}:` | `go-zero` CachedConn；GC 或旁路删除时需考虑缓存键失效（`shorturlmap_cachekeys.go`） |
| 发号器 | `shorturl:sequence`（可配置） | `INCR` 原子递增；可从 MySQL `sequence` bootstrap |
| 短链近似集合 | Cuckoo（RedisBloom）或 Legacy 布隆 | Show 前快速判断「不可能存在」→ 直接 404，减轻 DB；存在**假阳性**时再查库 |
| UV 辅助 | `stats:uv:{shortPath}:{day}` | `PFADD` HyperLogLog；配合日志明细做 UV 相关能力 |
| Asynq | 与配置中 Redis 地址一致 | 多队列：默认、reports、stats 等 |
| AI 报告任务 | `aireport.Store`（go-redis） | 任务状态、结果 JSON、用户编辑后的 Markdown；TTL 约 24h（见 `ServiceContext`） |

---

## 6. HTTP API 一览

以下路径与 `shorturl/shorturl.api`、`internal/handler/routes.go` 一致。`Host/Port` 以 `etc/shorturl-api.yaml` 为准（默认 `0.0.0.0:8888`）。

### 6.1 公开/半公开

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/convert` | 提交长链，返回短链展示串、过期时间、分类/安全/AI 建议等 |
| `GET` | `/:shortURL` | **解析短链**：成功则 **302** 到 `longURL`；失败返回错误（如 404、已过期） |

### 6.2 管理/运营（配置 `Admin.ApiToken` 非空时，需 `X-Admin-Token`）

与 `AdminAuthMiddleware` 一致：保护 `/admin/*`、`/stats`、`/analyze` 及子路径、`/links*`。

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/stats` | Query：`shortURL`, `startDate`, `endDate`（YYYY-MM-DD）；返回总 PV/UV、按日曲线、设备、地理聚合等 |
| `POST` | `/analyze` | Body：同上日期范围；**立即返回**与 `/stats` 一致的统计 + **异步 AI 报告** `reportJob`（jobId） |
| `GET` | `/analyze/report/status` | Query：`jobId`；轮询报告生成状态与结果 |
| `PUT` | `/analyze/report` | 保存用户编辑后的 Markdown |
| `GET` | `/links` | 分页列表，可选 `category` |
| `GET` | `/links/categories` | 已有分类去重列表 |
| `PUT` | `/links/:id` | 更新长链、分类、过期策略等 |
| `DELETE` | `/links/:id` | 删除（软删，视业务/GC） |
| `GET` | `/admin/performance` | 主机与 MySQL、Redis 连接与健康快照 |

**前端约定**（`shortUrlfrontend/src/api/shorturl.ts`）：开发环境 `baseURL` 为 `/api`，Vite 去掉前缀转发到后端；对 `/stats`、`/analyze`、`/links`、`/admin/` 自动附加 `X-Admin-Token`（当配置了 `VITE_ADMIN_API_TOKEN`）。

---

## 7. 核心业务流程

### 7.1 转链 `POST /convert`（`ConverLogic`）

1. **连通性探测**：对用户长链做受 SSRF 策略约束的 HTTP 请求；非成功响应则拒绝。
2. **同 MD5 已存在**：区分有效 / 已过期 / 已软删 —— 幂等返回或 **UPDATE 续约/复活**（不能新建行，否则违反 `unique(lurl)`）。
3. **防套娃**：若长链基底路径已是已有 `surl`，拒绝。
4. **生成 `surl`**：自定义（黑名单 + 占用检查）或 **Sequence + Base62**（命中黑名单则循环重取）。
5. **过期**：支持 `ExpirePreset`、`ExpireAfter*`、`ExpireAt`（RFC3339），全无则 `expire_at` 为 NULL。
6. **可选 AI**：Fetcher 拉取页面 + 模型分析；若判为危险可拒绝写入。
7. **持久化成功后** **best-effort** 写入近似过滤器（先 DB 后过滤器，避免插入失败却污染过滤器）。

### 7.2 跳转 `GET /:shortURL`（`ShowHandler` + `ShowLogic`）

1. **过滤器**：不存在则直接 404（防缓存穿透）；存在则查 DB。
2. **软删 / 过期**：404 或「链接已过期」类错误。
3. **异步访问日志**：封装 `AccessLogTask`，经 Asynq 入库 `access_log`；Worker 内可对缺省国家城市做 **GeoIP** 补全。
4. **Redis HLL**：对 IP 做 `PFADD` 辅助 UV 相关统计（与日志明细并存）。
5. 响应：**302 Redirect** 至长链（非 JSON）。

### 7.3 统计 `GET /stats`

- 全部基于 **`access_log` 区间聚合**，保证总 PV、按日 PV、设备、地理等口径一致。
- `access_stats` 由定时任务维护，**不用于该接口**，避免延迟与不一致。

### 7.4 分析 `POST /analyze` + 报告轮询

1. 先执行与 `/stats` 相同的统计逻辑。
2. 生成 `jobId`，**Redis** 中创建 pending 记录；向 Asynq **reports 队列**投递 AI 报告任务（携带统计 JSON）。
3. 客户端通过 `/analyze/report/status` 轮询；可选用 **PUT `/analyze/report`** 保存人工编辑的 Markdown。

### 7.5 后台任务（`shorturl.go`）

- **Asynq Server**：处理访问日志、`stats:aggregate:hour`、AI 报告、可选 GC。
- **Scheduler**：每小时触发统计聚合；若开启 GC，每日按保留天数清理长期软删记录（并同步过滤器认知见配置注释）。

---

## 8. 实现细节摘选

- **go-zero 中间件**：`Timeout`、`Recover`、`Metrics`、`Prometheus`、`Trace`、`Log`、`MaxBytes`、`Gunzip` 等可在 yaml 中开关；另可挂载 **按 IP 令牌桶**（`ratelimitmiddleware.go`）与 **管理端 Token**。
- **SSRF**：对用户 URL 的探测与抓取统一 `ssrf` 包策略（端口、私网、重定向上限、Body 上限）；生产建议 `OnlyStdPorts=true`、`AllowPrivateTargets=false`。
- **GeoIP**：请求头可带 CDN 地理头；否则 Worker 侧按 IP 调可配置 HTTP API（默认示例为 ip-api），带缓存 TTL。
- **AI Fallback**：`internal/ai/report_fallback.go` 等在主线路失败时保证可读的降级输出。
- **短链黑名单**：保留路径如 `convert`、`stats`、`links` 等与路由冲突的片段不可用作短码。

---

## 9. 前端页面与路由

| 路由 | 视图 | 角色 |
|------|------|------|
| `/` | `Convert.vue` | 访客：生成短链 |
| `/entry/admin-portal-9x7k` | `AdminLogin.vue` | 管理登录（本地凭证写在前端配置，仅 UI 门槛） |
| `/admin/links` | `LinkList.vue` | 链接管理 |
| `/admin/analytics` | `Analytics.vue` | 统计 + 异步 AI 报告与轮询 |
| `/admin/performance` | `Performance.vue` | 性能快照 |

**说明**：管理后台「登录」与后端 `X-Admin-Token` 是两套机制：前者为前端路由守卫；后者为真实 API 保护，生产应对齐环境变量与 `Admin.ApiToken`。

---

## 10. 项目亮点

1. **go-zero 分层**：`api` 定义 → `handler` → `logic` → `model`/`pkg`，职责清晰，易测试与扩展。
2. **发号**：Redis **INCR** 替代 MySQL 热点发号，并可 **bootstrap** 旧库 `sequence`。
3. **防穿透**：Cuckoo/Bloom **前置拦截** 不存在短码；与 DB 真源分工明确。
4. **统计一致性**：在线 **只信 `access_log`**，避免汇总表与明细不同步。
5. **异步解耦**：访问日志、小时聚合、AI 报告均走 Asynq，主请求快速返回。
6. **安全纵深**：SSRF 策略、AI 安全拒绝、可选限流与管理 Token。
7. **可观测**：Prometheus、链路追踪、性能面板、MySQL/Redis ping 与状态。

---

## 11. 核心难点与权衡

| 难点 | 说明 |
|------|------|
| 过滤器与 DB 一致 | 布隆/Cuckoo **无法安全删元素**（或成本高）；软删 GC 后可能存在**假阳性**需再打 DB；新增短码需 **warmup** 与 best-effort 写入 |
| 唯一约束与并发 | `lurl`/`surl` 唯一；高并发下依赖 DB 与业务层「先查后写」的幂等与冲突恢复（如 `recoverFromDuplicateLurl`） |
| 长链路请求 | `/convert` 含探测、抓取、AI，需更大 **Timeout**（配置中已给到 180s 量级思路） |
| 多端口径 | UV 定义（IP 去重 vs HLL）、CDN 真实 IP、Geo 精度，需在运维与产品上达成一致 |
| 密钥与演示配置 | 配置文件中勿提交真实 **API Key**；使用环境变量或本地覆盖 yaml |

---

## 12. 本地运行依赖（参考）

- `docker-compose.yaml` 启动 **MySQL（13307）** 与 **Redis（16379，密码示例为 123456）**。
- 执行 `short_url_map.sql`、`sequence.sql`、`migration.sql` 初始化库表（按实际库名 `shorturl`）。
- 后端：`cd shorturl && go run shorturl.go -f etc/shorturl-api.yaml`。
- 前端：`cd shortUrlfrontend && npm install && npm run dev`；默认通过 `/api` 代理访问后端。

---

## 13. 文档修订说明

本文档随仓库迭代维护；若 `shorturl.api` 或 `migration.sql` 变更，请同步更新本章 API 与表结构描述。

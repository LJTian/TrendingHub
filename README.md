# TrendingHub (Go 热点聚合服务)

TrendingHub 是一个使用 Go 开发的「多源热点聚合」服务示例，目标是统一抓取和聚合多个站点（如微博、知乎、GitHub、Hacker News、V2EX 等）的热门内容，并通过统一的 RESTful API 对外提供访问。

当前仓库实现的是一个 **可运行的 MVP**，包含：

- Collector 层：定义统一的抓取接口，并实现 GitHub Trending 的真实抓取逻辑（基于 Colly）。
- Processor 层：做基础的数据规范化与去重入口。
- Storage 层：封装 PostgreSQL 与 Redis 访问。
- API Server 层：提供基础的新闻查询 API。
- Scheduler 层：使用 cron 定时触发抓取任务。

## 目录结构（规划）

- `cmd/api`：API 服务入口。
- `internal/config`：配置加载（环境变量、默认配置等）。
- `internal/collector`：各数据源抓取逻辑与统一 `Fetcher` 接口。
- `internal/processor`：数据清洗、去重、热度计算等。
- `internal/storage`：数据库与缓存访问封装。
- `internal/api`：Gin 路由与 Handler。
- `internal/scheduler`：定时任务调度。
- `web/`：前端单页应用（React + TypeScript + Vite）。

## 快速开始（开发环境）

1. 启动依赖服务（PostgreSQL + Redis）：

```bash
docker compose up -d
```

2. 启动 API 服务：

```bash
cd cmd/api
go run .
```

3. 启动前端页面（可选）：

```bash
cd web
npm install
npm run dev
```

前端默认运行在 `http://localhost:5173`，通过 Vite 代理访问后端 `http://localhost:9000`。

4. 等待定时任务（默认每 30 分钟）或手动触发一轮采集：

- 首次启动后，可以在日志中看到 `fetch GitHub Trending...` 与 `collect job done` 字样。

5. 测试基础接口（如果只用 API）：

- `GET http://localhost:9000/health`：健康检查。
- `GET http://localhost:9000/api/v1/news`：获取新闻列表，支持参数：
  - `channel`：数据源 code，例如 `github`。
  - `sort`：排序方式，`latest`（按发布时间倒序，默认）或 `hot`（按热度值倒序）。
  - `limit`：返回条数，默认 20，上限 100。

示例：

```bash
curl "http://localhost:9000/api/v1/news?channel=github&sort=hot&limit=10"
```

> 注意：
> - GitHub Trending 页面结构可能会变化，解析逻辑属于“尽力而为”的实现。
> - 若访问出现异常，可先在本机浏览器确认 `https://github.com/trending` 是否可访问，再查看服务端日志。

# TrendingHub
自动检索热点信息网站

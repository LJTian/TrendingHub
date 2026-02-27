# TrendingHub

多源热点聚合服务 —— 统一抓取 GitHub Trending、百度热搜、Hacker News、金融行情等热门数据，并提供天气预报，通过 Web 界面一站式浏览。

![TrendingHub 首页截图](image.png)

## 功能特性

- **首页仪表板**：总览所有频道的热门内容，一屏掌握全局
- **GitHub Trending**：抓取 GitHub 每日热门仓库，自动翻译非中文描述
- **百度热搜**：实时获取百度热搜榜单
- **Hacker News**：抓取 Hacker News 热门文章
- **金融行情**：黄金价格（元/克）；A 股三大指数（上证、深证、创业板）置顶，其它股票通过环境变量 `ASHARE_STOCK_CODES` 手动配置（逗号分隔 6 位代码，如 `600519,000858,300750`），不展示涨幅榜
- **天气预报**：基于 [QWeather 和风天气](https://dev.qweather.com/)，支持多城市标签页切换，当前天气 + 3 天预报
- **日期筛选**：支持按日期查看历史数据
- **定时采集**：各数据源按独立周期自动更新

## 技术栈

| 层 | 技术 |
|---|---|
| 前端 | React + TypeScript + Vite |
| 后端 | Go + Gin |
| 数据库 | PostgreSQL（GORM） |
| 缓存 | Redis |
| 定时任务 | robfig/cron/v3 |
| 采集 | Colly + 标准 HTTP |

## 目录结构

```
cmd/api/             API 服务入口
internal/
  api/               Gin 路由与 Handler（含天气代理）
  collector/         各数据源采集器
    github_mock.go     GitHub Trending
    baidu_hot.go       百度热搜
    hackernews.go      Hacker News
    gold_chart.go      黄金价格
    ashare_index.go    A 股指数
    translate.go       翻译工具
  config/            配置加载
  processor/         数据清洗与去重
  scheduler/         定时任务调度
  storage/           PostgreSQL + Redis 封装（含天气缓存）
web/                 前端 SPA（React + Vite）
```

## 快速开始

### 1. 启动依赖服务

```bash
docker compose up -d
```

### 2. 启动后端

```bash
cd cmd/api
go run .
```

启动时会自动：
- 初始化数据库表结构
- 添加默认天气城市（北京）并预取天气缓存
- 启动定时采集任务

### 3. 启动前端

```bash
cd web
npm install
npm run dev
```

前端默认运行在 `http://localhost:5173`，通过 Vite 代理访问后端 `http://localhost:9000`。

## 采集周期

| 数据源 | 周期 |
|--------|------|
| 百度热搜 | 每 30 分钟 |
| 黄金价格 | 每 30 分钟 |
| A 股指数 | 每 3 分钟 |
| Hacker News | 每小时 |
| GitHub Trending | 每 2 小时 |
| 天气数据 | 每小时 |

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/api/v1/news` | 新闻列表（参数：`channel`、`sort`、`limit`、`date`） |
| GET | `/api/v1/news/dates` | 有数据的日期列表 |
| GET | `/api/v1/weather` | 所有关注城市的天气缓存 |
| GET | `/api/v1/weather/cities` | 天气城市列表 |
| POST | `/api/v1/weather/cities` | 添加天气城市（body: `{"city":"城市名"}`) |
| DELETE | `/api/v1/weather/cities/:city` | 移除天气城市 |

示例：

```bash
curl "http://localhost:9000/api/v1/news?channel=github&sort=hot&limit=10"
curl "http://localhost:9000/api/v1/weather"
```

## 注意事项

- GitHub Trending 页面结构可能变化，解析逻辑属于"尽力而为"的实现
- 非中文仓库描述会自动翻译为中文（优先 Google 翻译，失败时回退 MyMemory）
- 天气数据来源于 QWeather 和风天气（需要申请免费开发者 Key，在运行环境中配置 `QWEATHER_API_KEY`、`QWEATHER_API_HOST`，例如使用 `.env` 文件或部署平台的环境变量功能），后端定时缓存确保响应速度
- 若希望为整个站点添加访问密码，可在运行环境中配置 `APP_BASIC_USER` 和 `APP_BASIC_PASS`，启用 HTTP Basic Auth 保护（浏览器会在访问时弹出账号/密码框；`/health` 接口不受影响）
- A 股自选股：设置环境变量 `ASHARE_STOCK_CODES`（逗号分隔，如 `600519,000858,300750`），金融频道会在三大指数下方展示这些股票的行情；不设置则仅展示黄金 + 三大指数
- X 热搜因外部数据源不稳定暂未接入，采集器代码保留在 `internal/collector/x_trends.go`

### 全站访问密码

若需要在生产环境为整站加上一层轻量的 HTTP Basic Auth，设置 `APP_BASIC_USER` 与 `APP_BASIC_PASS` 即可。配置完成后，Go 服务会拦截除 `/health` 以外的所有请求并触发浏览器的账号/密码弹窗；只要在环境中传入（例如 `docker compose` 文件会读取根目录 `.env` 中的变量），同一个域名下的 API 与静态页面都自动使用该凭据，不需要额外在前端里处理。

**启用示例**：

```bash
# .env（与 docker-compose.yml 放在同一目录）
APP_BASIC_USER=admin
APP_BASIC_PASS=secret

# 然后重新启动容器
docker compose down
docker compose up -d
```

如需在其它部署方式下启用，只需将上述两个环境变量注入运行 `trendinghub` 可执行文件即可。

### CI / CD 与容器镜像发布

项目提供两个现成的 GitHub Actions 工作流：

- `CI`（`.github/workflows/ci.yml`）：对 `main` 分支的 push/PR，先运行 Go 的单元测试，然后进入 `web` 目录构建 Vite 静态资源，确保前后端都能在干净环境下编译。
- `Publish Container`（`.github/workflows/docker-publish.yml`）：基于 Docker Buildx 构建 `linux/amd64` 和 `linux/arm64` 镜像，并推送到容器仓库，默认命名为 `docker.io/ljtian/trendinghub`，支持手动触发（`workflow_dispatch`）或每次向 `main` push。

要让镜像成功推送，需要在仓库设置以下 Secrets：

| 名称 | 说明 |
|------|------|
| `CONTAINER_REGISTRY_USERNAME` | 容器仓库登录用户名（Docker Hub、GitHub Packages、ArgoCD Registry 等） |
| `CONTAINER_REGISTRY_PASSWORD` | 对应的访问凭证（Docker Hub Access Token、GitHub Personal Access Token、或 Registry 密码） |
| `CONTAINER_REGISTRY_URL`（可选） | 镜像仓库地址，默认 `docker.io`。将镜像推送到其它地址时填如 `ghcr.io` |
| `CONTAINER_REGISTRY_IMAGE`（可选） | 镜像名（包含仓库），默认 `ljtian/trendinghub`。例如 `myorg/trendinghub` |

Workflow 会使用上述凭证登录，在 `Dockerfile` 所在目录构建并同时推送 `latest` 与 `${{ github.sha }}` 标签，拿到不同平台的镜像都可以直接拉取或用于后续 CD。

### 发布分支策略

为了把镜像交付和测试流程串联在一起，仓库专门提供一个 `release/*` 分支供正式发布用：

- 只要推送 `release/xxx`（或在 Actions 页面手动触发 `Release Pipeline`），工作流会先执行完整的自动化测试（Go 单元测试 + 前端打包），与主分支 `CI` 流程完全一致；
- 测试成功后续 job 会构建 `linux/amd64` 与 `linux/arm64` 镜像，并推送 `latest`、`release-xxx`（命名里把 `/` 替换为 `-`）以及 `${{ github.sha }}` 标签到容器仓库；
- 镜像推送仍然复用 `CONTAINER_REGISTRY_*` secrets，不需要额外配置；
- 如果你希望在 release 之后做额外的部署（比如 GitHub Releases、Kubernetes rollouts），可以再接一个依赖 release 镜像标签的工作流。

在本地执行 `git switch -c release/1.0 && git push origin release/1.0` 就能触发上述流程，测试通过后镜像会自动上传，后续可以直接拉取 `ghcr.io/myorg/trendinghub:release-1.0`（或你设定的仓库）。

### 本地统一验证命令

项目根目录新增了 `Makefile`，封装了 CI/Release 里相同的验证步骤：

```bash
make test
```

该命令依次执行 `go test ./...`、`npm ci`（在 `web` 下）与 `npm run build`，如果你希望仅跑 Go、前端单独构建也可以使用 `make backend-test` 或 `make frontend-build`（后者会在构建前自动安装依赖）。

### 本地 CI / CD 验证流程

在推动到 GitHub 之前可以本地模拟 CI/CD：

- **本地 CI**：使用 `make test`（或者直接 `go test ./...` + `cd web && npm ci && npm run build`）来验证后端测试和前端打包，和 `.github/workflows/ci.yml` / `.github/workflows/release.yml` 中的 `tests` job 保持一致。
- **本地 CD（容器构建）**：先保证 `docker buildx` 可用，再运行：
  ```bash
  docker buildx build --platform linux/amd64,linux/arm64 \
    -t ${REGISTRY:-docker.io}/${IMAGE:-ljtian/trendinghub}:local \
    --push .
  ```
 这里会复用 CI 中的多个标签逻辑（可改成 `--load` 仅用于本地测试），登录凭证可以提前用 `docker login` 或设置 `CONTAINER_REGISTRY_USERNAME` / `CONTAINER_REGISTRY_PASSWORD` 环境变量。
- **本地 Release 预演**：如果要模拟 `release/*` 分支的镜像标签逻辑，可在 push 前设置 `GITHUB_REF_NAME=release/1.0`、`GITHUB_SHA=$(git rev-parse HEAD)`，再从 `publish` job 中复制 `docker buildx` 相关命令（或者用 `make` 结合 `REGISTRY=... IMAGE=...` 手动调用）。

这样一来无需每次推送都等待云端运行 CI/CD，就可以在开发机先对关键流程做一次完整验证。

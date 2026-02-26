# TrendingHub 项目审计报告

审计日期：2025-02-26  
项目路径：`/Users/ljtian/data/git/github.com/LJTian/TrendingHub`

---

## 一、项目概览

多源热点聚合服务：Go + Gin 后端，React + TypeScript 前端，PostgreSQL + Redis，定时采集 GitHub Trending、百度热搜、Hacker News、金融行情、和风天气等。审计范围包括安全、健壮性、API 设计与部署配置。

---

## 二、安全审计

### 2.1 已做得较好的部分

| 项目 | 说明 |
|------|------|
| **Basic Auth** | `main.go` 使用 `crypto/subtle.ConstantTimeCompare` 做账号密码比对，避免时序攻击；`/health` 豁免便于健康检查。 |
| **SQL 注入** | 所有用户输入（channel、date、city、code、limit 等）均通过 GORM 参数化或来自固定映射的表名（`sourceToTable`），未发现拼接 SQL 的用户可控输入。 |
| **XSS** | 前端未使用 `dangerouslySetInnerHTML`，React 默认对文本转义，新闻标题/描述以文本展示，XSS 风险低。 |
| **SSRF（金价）** | `gold_chart.go` 对 `GOLD_API_URL` 做 host 白名单（`data-asg.goldprice.org` / `data-goldprice.org`）且仅允许 https，避免任意 URL 请求。 |
| **敏感配置** | 数据库 DSN、Redis、QWeather API Key、Basic Auth 均从环境变量读取，未硬编码。 |

### 2.2 建议与风险点

- **生产环境密码**：`docker-compose.yml` 中 `POSTGRES_PASSWORD` 等为默认值，生产部署务必修改并为 DB/Redis 使用强密码或 secrets。
- **API 无鉴权细分**：开启 Basic Auth 后整站统一保护；若未来需要“仅写操作需登录”，可对 `POST/DELETE /api/v1/weather/cities`、`POST/DELETE /api/v1/ashare/stocks` 单独加中间件。
- **CORS**：当前未显式配置 CORS；若前后端分离部署且前端域名与 API 不同源，需在 Gin 中配置 `AllowOrigins` 等，避免跨域被浏览器拦截。
- **限流**：`POST /api/v1/weather/cities` 与 `POST /api/v1/ashare/stocks` 无速率限制，恶意客户端可频繁添加城市/自选股。建议对写接口做 IP 或 Basic Auth 维度的限流（如 gin 限流中间件）。

---

## 三、逻辑与健壮性（已修复与建议）

### 3.1 已修复

1. **`internal/collector/ashare_index.go` — `codeToSecID` 空字符串**
   - **问题**：`len(code) < 1` 时返回 `"0." + code`，即 `"0."`，作为东方财富 `secid` 无效，且语义错误。
   - **修复**：空 `code` 时返回 `""`；在 `fetchOneStock` 开头对 `code` 与 `codeToSecID(code)` 做空检查，空则直接返回 `nil`。

2. **`internal/scheduler/scheduler.go` — 采集 panic 导致进程退出**
   - **问题**：`runFetcher` 在 goroutine 中调用 `f.Fetch()`，若某采集器 panic（如解析异常、第三方库问题），整个 goroutine 崩溃，`RunOnce` 中并发执行时会导致进程退出。
   - **修复**：在 `runFetcher` 开头增加 `defer recover()`，将 panic 转为打日志，不影响其他采集器与主进程。

### 3.2 建议（未改代码）

- **添加城市后的天气拉取**：`addWeatherCity` 在 goroutine 中异步拉取天气并写缓存，失败仅打 log，用户已收到 200。若希望“添加后立即可见”，可考虑短轮询或前端在添加后延迟几秒再请求列表；或在响应中提示“正在获取天气，请稍后刷新”。
- **`removeWeatherCity` 的 city 参数**：当前仅要求非空，未限制长度或字符集。GORM 参数化无注入风险，但可对 path 中 city 做与 `addWeatherCity` 一致的长度/字符集约束，避免异常值入库或日志膨胀。

---

## 四、API 与输入校验

- **`listNews`**：`limit` 有上界（默认 100，gold 渠道 600），`date` 校验 `2006-01-02`，`sort` 限定 `latest`/`hot`，`channel` 由存储层按 `sourceToTable` 映射，设计合理。
- **`listNewsDates`**：`limit` 最大 365，`channel` 同上。
- **天气城市**：`addWeatherCity` 对 body 中 `city` 按 rune 截断至 30，避免过长；`removeWeatherCity` 使用 path 参数，建议同上节做长度/字符集约束。
- **A 股自选股**：`addAshareStock` 使用 `storage.NormalizeStockCode`，仅接受 6 位数字；`removeAshareStock` 的 path `code` 会先 `NormalizeStockCode`，非数字则用 TrimSpace 后删除，行为明确。

---

## 五、依赖与运维

- **采集器**：百度、黄金、东方财富、翻译等均设置 HTTP 超时与响应体大小限制（如 2MB、256KB），避免长时间阻塞或内存耗尽。
- **数据库**：`NewStore` 有重试与 AutoMigrate；分表名来自常量与映射，无用户输入。
- **日志**：未发现打印 API Key 或密码；错误信息为通用或可接受级别。
- **Docker**：`docker-compose` 中 postgres 有 healthcheck，app 依赖 healthy 后再启动，逻辑正确；生产需替换默认密码并考虑 Redis 持久化与备份策略。

---

## 六、审计结论与修复汇总

| 类别 | 结论 |
|------|------|
| **安全** | 无 SQL 注入、XSS、SSRF（金价已白名单）；Basic Auth 实现正确；生产需改默认密码并可按需加 CORS、限流。 |
| **健壮性** | 已修复 A 股空 code 边界与采集器 panic 导致进程退出；其余为建议优化。 |
| **代码质量** | 结构清晰，配置与采集分离明确，存储层参数化一致；建议对写接口和 path 参数做适度约束与限流。 |

**本次已修改文件：**

- `internal/collector/ashare_index.go`：`codeToSecID` 空 code 返回 `""`，`fetchOneStock` 增加空 code/secID 早退。
- `internal/scheduler/scheduler.go`：`runFetcher` 增加 `defer recover()`，避免单源 panic 导致进程退出。

以上修改已通过当前 linter 检查，建议在本地跑一遍采集与 A 股/天气相关接口的回归测试。

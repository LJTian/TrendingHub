# TrendingHub 代码审计报告

**审计日期**: 2025-02-10  
**项目路径**: `/Users/ljtian/data/git/github.com/LJTian/TrendingHub`

---

## 一、项目概览

TrendingHub 是一个多源热点聚合服务（Go + React），包含采集器、处理器、存储、调度器与 REST API。审计覆盖后端 `cmd/`、`internal/` 及前端 `web/src`、配置与依赖。

---

## 二、发现的问题与建议

### 1. 【逻辑错误】API `limit` 与文档/存储不一致

- **位置**: `internal/api/router.go`、`internal/storage/storage.go`
- **现象**: README 约定「limit 上限 100」；API 层未对 `limit` 做上限校验，仅对解析失败或 ≤0 时设为 20；Storage 将 `limit > 1000` 时才截断为 20。
- **影响**: 调用方可传 `limit=500` 等，与文档不符，且单次查询可能返回过多数据。
- **建议**: 在 API 层统一将 `limit` 限制在 1～100（与 README 一致），Storage 保留 1000 作为安全上限。

---

### 2. 【逻辑错误】`channel=gold` 时忽略用户 `limit`

- **位置**: `internal/storage/storage.go` 中 `ListNews` 的 `channel == "gold"` 分支
- **现象**: 金融渠道固定使用 `Limit(500)`（黄金）和 `Limit(20)`（A 股），未使用参数 `limit`。
- **影响**: 用户请求 `limit=10` 时仍可能收到 500+20 条，行为与接口约定不符。
- **建议**: 在合并 gold + ashare 后对 `list` 做 `list = list[:min(len(list), limit)]`，或分别按比例/上限取子集再合并，使总条数不超过 `limit`。

---

### 3. 【健壮性】前端未处理 `data` 缺失

- **位置**: `web/src/api.ts` 中 `return json.data`
- **现象**: 若服务端返回 `{ code: "ok" }` 但未带 `data` 字段，`json.data` 为 `undefined`，调用方可能假设始终为数组而报错。
- **建议**: 使用 `return json.data ?? []` 或对缺失 `data` 时抛出明确错误，避免下游使用 `undefined`。

---

### 4. 【安全/配置】外部 URL 可被环境变量覆盖

- **位置**: `internal/collector/gold_chart.go` 中 `GOLD_API_URL`
- **现象**: 完全信任环境变量，若被设置为内网或恶意 URL，可能造成 SSRF。
- **建议**: 生产环境通过白名单校验（如仅允许 `https://data-asg.goldprice.org` 等）；或仅在文档中明确「仅可信环境可覆盖该变量」。

---

### 5. 【健壮性】HTTP 响应体未限制大小

- **位置**: `internal/collector/ashare_index.go`、`internal/collector/x_trends.go` 等使用 `io.ReadAll(resp.Body)`
- **现象**: 若上游返回超大响应，会一次性读入内存，存在 DoS 风险。
- **建议**: 使用 `io.LimitReader(resp.Body, maxBytes)`（如 512KB～1MB）再读取，超出部分丢弃并记录或返回错误。

---

### 6. 【可维护性】Processor 使用 SHA1 生成 ID

- **位置**: `internal/processor/processor.go` 中 `hashURL`
- **说明**: 当前仅用于去重与主键，无密码学安全要求，SHA1 在此场景可接受；若未来用于安全相关场景，应改为 SHA256 等。
- **建议**: 保持现状即可，可在注释中注明「仅用于去重 ID，非安全用途」。

---

### 7. 【运维】默认配置含明文密码

- **位置**: `internal/config/config.go` 中默认 `PostgresDSN`
- **说明**: 默认值包含 `password=trendinghub`，适合本地开发；生产应通过环境变量覆盖且不提交敏感值。
- **建议**: 在 README 或部署文档中明确「生产必须设置 POSTGRES_DSN 等环境变量，勿使用默认 DSN」。

---

### 8. 【一致性】Redis 连接失败不中断启动

- **位置**: `internal/storage/storage.go` 中 `NewStore` 对 `rdb.Ping` 失败仅打日志
- **说明**: 服务仍会启动，后续请求会回退到 DB，属于容错设计；若希望「无 Redis 不启动」，可改为 `return nil, err`。
- **建议**: 保持当前行为即可，在文档中说明「Redis 可选，失败时仅不使用缓存」。

---

### 9. 【性能】SaveBatch 每条 2 次 DB 往返

- **位置**: `internal/storage/storage.go` 中 `SaveBatch`
- **现象**: 每条记录先 `FirstOrCreate` 再 `Updates`，新建记录时 Updates 与 Create 数据重复，多一次写。
- **建议**: 可优化为「仅当 First 成功（已存在）时再 Updates」或使用批量 Upsert（若 GORM/Postgres 支持），减少 round-trip。

---

### 10. 【部署】生产 CORS

- **位置**: `cmd/api/main.go` 使用 `gin.Default()`，未配置 CORS
- **说明**: 开发时通过 Vite 代理同源访问无问题；若前端与 API 不同域部署，需在 Gin 中加 CORS 中间件。
- **建议**: 生产若前后端分离部署，增加 CORS 配置并限制允许的 Origin。

---

## 三、已确认良好的实践

- **SQL 安全**: 使用 GORM 参数化查询（如 `Where("source = ?", channel)`），未发现拼接 SQL。
- **采集限域**: Colly 使用 `AllowedDomains`、合理 User-Agent 与超时，降低爬虫滥用与挂起风险。
- **翻译输入**: `translate.go` 对长度截断（500）、`url.QueryEscape` 使用正确。
- **缓存**: ListNews 使用 Redis 缓存并设置 60s TTL，回写前校验 `len(list) > 0`，逻辑清晰。
- **错误处理**: 采集器单源失败不拖垮整轮任务；API 对错误返回统一结构。

---

## 四、修复优先级建议

| 优先级 | 项 | 说明 |
|--------|----|------|
| 高 | #1 limit 上限 | 与文档一致，避免单次过大查询 |
| 高 | #2 gold 渠道 limit | 接口行为符合约定 |
| 中 | #3 前端 data 兜底 | 避免 undefined 导致前端异常 |
| 中 | #5 响应体大小限制 | 降低 DoS 风险 |
| 低 | #4 URL 白名单 | 生产环境建议 |
| 低 | #9 SaveBatch 优化 | 性能优化，非正确性 |

---

## 五、总结

项目结构清晰，采集/处理/存储/API 分层合理，未发现严重安全漏洞或明显逻辑谬误。主要问题集中在 **API limit 语义与文档不一致**、**gold 渠道未应用 limit** 以及 **前端与 HTTP 响应体健壮性**。按上述优先级修复后，可提升一致性、可预测性与抗 DoS 能力。

---

## 六、继续审计（第二轮）— 已实施修复

### 6.1 审计报告中未完成的项

| 项 | 处理 |
|----|------|
| **#5 响应体大小限制** | 已在 `ashare_index.go`（1MB）、`x_trends.go`（2MB）、`gold_chart.go`（64KB）、`translate.go`（256KB）中使用 `io.LimitReader(resp.Body, maxBytes)`，避免超大响应 DoS。 |
| **#6 SHA1 注释** | 已在 `processor.go` 的 `hashURL` 上增加注释：「仅用于去重与主键生成，非密码学用途」。 |
| **#4 GOLD_API_URL 白名单** | 已在 `gold_chart.go` 中增加 `isAllowedGoldAPIURL`：仅允许 `https` 且 host 为 `data-asg.goldprice.org` 或 `data-goldprice.org`；否则回退默认 URL 并打日志。 |

### 6.2 新发现并已修复

- **前端 gold 请求 limit 与 API 不一致**  
  - **位置**: `web/src/App.tsx` 中 `limit: isGold ? 500 : 30`。  
  - **现象**: API 已限制 limit 上限 100，前端仍请求 500，语义冗余且易误导。  
  - **修复**: 改为 `limit: isGold ? 100 : 30`，与 API 约定一致。

### 6.3 仍建议后续处理

- **#7 / #8**: 在 README 或部署文档中说明「生产须设置 POSTGRES_DSN」「Redis 可选，失败时仅不用缓存」。  
- **#9 SaveBatch**: 仍可做「仅存在时 Update」或批量 Upsert 优化。  
- **#10 CORS**: 生产若前后端分离部署，需在 Gin 中增加 CORS 中间件。

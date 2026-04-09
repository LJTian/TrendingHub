# Product Hunt 检索接入设计

## 背景

TrendingHub 目前已经聚合了 GitHub Trending、百度热搜、Hacker News、金融行情等频道，但还没有 Product Hunt 数据源。用户希望在 Product Hunt 场景下获得“可持续同步、可站内检索、可按标签筛选”的浏览能力，而不是仅仅跳转外部网站。

## 目标

1. 将 Product Hunt 作为一个新的频道接入 TrendingHub。
2. 通过后端定时同步，把热门产品数据落到本地数据库。
3. 在站内支持关键词检索、标签筛选和常规排序。
4. 尽量复用现有 `news` 模型、频道页和日期筛选逻辑，控制改动范围。

## 非目标

1. 不做对 Product Hunt 的实时代理抓取。
2. 不做独立的全文搜索服务或外部搜索引擎集成。
3. 不重构现有新闻体系的整体模型，只在现有结构上扩展。

## 方案概述

采用“同步入库 + 本地检索”的方案：

1. 后端新增 `producthunt` 采集器，定时拉取 Product Hunt 热门流。
2. 将采集结果转换为现有统一结构后写入 `news_producthunt` 分表。
3. API 在现有 `/api/v1/news` 上扩展查询参数，支持 `q` 和 `tag`。
4. 前端在频道页顶部增加搜索框和标签筛选入口，复用当前列表渲染。

这个方案的优点是实现简单、依赖少、可维护性高，并且与 TrendingHub 现有“多源热点聚合”的产品定位一致。

## 数据来源策略

优先使用 Product Hunt 官方 API / GraphQL；如果没有可用 token 或接口能力受限，则回退到抓取公开页面。

设计上将数据获取逻辑封装在独立采集器内，不让上层 API 感知具体来源。这样后续即使切换抓取方式，也不影响前端和数据库结构。

## 数据模型

继续沿用现有 `News` 结构，并为 Product Hunt 补充必要的原始字段到 `extra_data` 中：

- `productId`
- `slug`
- `tagline`
- `topics`
- `votesCount`
- `commentsCount`
- `makerNames`

`title` 使用产品名称，`description` 使用 tagline 或简介，`url` 指向产品详情页。`hotScore` 采用投票数或一个可解释的热度组合值，便于与现有频道统一排序。

## 后端设计

### 采集器

新增 `internal/collector/producthunt.go`，实现 `Fetcher` 接口。采集器负责：

- 拉取热门产品列表
- 过滤掉空标题、无效链接和重复项
- 标准化时间、热度和标签
- 将原始字段放入 `RawData`

### 存储

在 `internal/storage/storage.go` 中新增：

- `allowedSources` 增加 `producthunt`
- `sourceToTable` 增加 `news_producthunt`
- `AutoMigrate` 后创建对应分表

Product Hunt 数据单独放表，可以避免和现有频道互相影响，也方便按频道做缓存和查询优化。

### 查询

在 `ListNews` 中扩展查询能力，支持：

- `q`：对标题、描述、标签做模糊检索
- `tag`：按标签过滤
- `sort=hot|latest`：继续复用现有排序
- `channel=producthunt`：只查 Product Hunt 数据

第一版优先采用 PostgreSQL `ILIKE` 和 JSON 字段过滤，避免为了检索再引入额外组件。若后续数据量明显增长，再升级为全文索引或 trigram 索引。

## API 设计

现有 `/api/v1/news` 保持兼容，只是增加可选参数：

- `channel=producthunt`
- `q=search text`
- `tag=ai`
- `sort=hot`
- `limit=20`
- `date=YYYY-MM-DD`

前端无需新建一套接口协议，只要沿用现有 `fetchNews` 即可。

## 前端设计

在频道页增加一个搜索区：

- 关键词输入框
- 标签快捷筛选
- 清空条件按钮

默认情况下仍然展示 Product Hunt 热门列表。输入关键词后，列表即时刷新并展示匹配结果。卡片展示保留当前风格，增加以下信息：

- 产品名称
- tagline / 简介
- 投票数
- 标签
- maker 或发布者信息

## 调度与缓存

采集周期建议先设为 30 分钟到 1 小时之间，默认 30 分钟更适合热点频道。若官方 API 受限或失败率上升，可自动退回到更保守的周期，并保留最近一次成功结果供站内查询。

缓存策略保持和现有项目一致：

- 查询结果进入 Redis，减少重复数据库读取
- 采集失败时不清空旧数据
- 采集器失败只影响 Product Hunt，不影响其他频道

## 错误处理

1. 上游 API 失败时记录日志并保留上一次成功数据。
2. 解析到脏数据时跳过单条记录，不让整个批次失败。
3. 搜索条件为空时回退到默认热门排序。
4. 如果标签字段不存在，检索只按标题和描述执行，不返回错误。

## 测试计划

1. 采集器单测：覆盖正常返回、空列表、字段缺失和重复数据。
2. 存储层测试：覆盖 `channel=producthunt` 的入库、查询、关键词过滤。
3. API 测试：覆盖 `q`、`tag`、`sort` 参数组合。
4. 前端构建：确认新增搜索栏不破坏现有频道页。

## 验收标准

1. 可以在 TrendingHub 中看到 `Product Hunt` 频道。
2. Product Hunt 数据会按固定周期同步到本地。
3. 用户可以按关键词或标签搜索 Product Hunt 条目。
4. 现有频道、日期筛选和金融页功能不受影响。
5. 没有新增外部搜索依赖，部署方式保持不变。

## 风险与权衡

- Product Hunt 页面或接口结构可能变化，因此采集器必须容忍字段缺失。
- 仅靠 `ILIKE` 的检索能力在早期足够，但数据量增大后性能会下降。
- 如果官方 API 需要额外授权，部署时需要明确配置方式，否则只能依赖抓取回退。

## 待确认事项

1. Product Hunt 频道的默认排序是否以 `hot` 为主。
2. 标签筛选是否需要支持多选。
3. 是否需要把 Product Hunt 默认纳入首页总览卡片。


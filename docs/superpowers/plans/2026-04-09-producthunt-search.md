# Product Hunt Search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Product Hunt as a first-class TrendingHub channel with scheduled sync, local search, and tag filtering.

**Architecture:** Extend the existing `news` pipeline instead of introducing a parallel system. A new Product Hunt fetcher will normalize upstream items into the shared `collector -> processor -> storage -> API -> web` flow, while `ListNews` gains keyword/tag filters that the UI can pass through unchanged.

**Tech Stack:** Go 1.24, Gin, GORM, PostgreSQL 16, Redis 7, React 18, TypeScript, Vite, robfig/cron/v3.

---

## File Map

- `internal/config/config.go` and `internal/config/config_test.go` own Product Hunt feature flags, cron defaults, and optional API token loading.
- `cmd/api/main.go` owns channel registration and scheduler job wiring.
- `internal/collector/producthunt.go` owns upstream fetching, normalization, and raw metadata packaging.
- `internal/collector/producthunt_test.go` owns parser/normalizer tests for Product Hunt payloads.
- `internal/storage/storage.go` owns schema registration, channel table mapping, and `ListNews` search filtering.
- `internal/storage/storage_test.go` owns persistence/query integration checks for `producthunt`.
- `internal/api/router.go` owns request parameter parsing and validation.
- `web/src/api.ts`, `web/src/App.tsx`, `web/src/types.ts`, and `web/src/styles.css` own the search request contract and UI.
- `README.md` owns the user-facing docs for the new channel and search parameters.

## Task 1: Wire Product Hunt into config and job registration

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/api/main.go`
- Modify: `README.md`

- [ ] **Step 1: Write the failing config test**

```go
func TestLoadEnablesProductHuntDefaults(t *testing.T) {
	_ = os.Unsetenv("PRODUCTHUNT_API_TOKEN")

	cfg := Load()
	if !cfg.EnableProductHunt {
		t.Fatalf("EnableProductHunt = false, want true")
	}
	if cfg.ProductHuntCron == "" {
		t.Fatalf("ProductHuntCron should have a default value")
	}
}
```

- [ ] **Step 2: Run the config test and verify it fails**

Run: `go test ./internal/config -run TestLoadEnablesProductHuntDefaults -v`

Expected: FAIL because `EnableProductHunt` and `ProductHuntCron` do not exist yet.

- [ ] **Step 3: Add the minimal config and main wiring**

```go
// internal/config/config.go
EnableProductHunt bool
ProductHuntCron   string
ProductHuntAPIToken string

// defaults
EnableProductHunt: true,
ProductHuntCron:   "*/30 * * * *",
ProductHuntAPIToken: getEnv("PRODUCTHUNT_API_TOKEN", ""),
```

```go
// cmd/api/main.go
if _, err := store.EnsureChannel("producthunt", "Product Hunt", "https://www.producthunt.com/"); err != nil {
	log.Fatalf("ensure channel producthunt failed: %v", err)
}
if cfg.EnableProductHunt {
	jobs = append(jobs, scheduler.FetcherJob{
		Fetcher:  &collector.ProductHuntFetcher{APIToken: cfg.ProductHuntAPIToken},
		CronSpec: cfg.ProductHuntCron,
	})
}
```

- [ ] **Step 4: Re-run the config test and a full config test file**

Run: `go test ./internal/config -v`

Expected: PASS.

- [ ] **Step 5: Update the docs**

Add Product Hunt to the feature list, env var table, and采集周期 table in `README.md`.

## Task 2: Build the Product Hunt fetcher with pure parsing tests

**Files:**
- Create: `internal/collector/producthunt.go`
- Create: `internal/collector/producthunt_test.go`
- Modify: `internal/collector/fetcher.go` if a small helper is needed for shared types

- [ ] **Step 1: Write a parser-focused failing test**

```go
func TestNormalizeProductHuntItem(t *testing.T) {
	edge := productHuntEdge{
		Node: productHuntNode{
			Name:    "Cursor",
			URL:     "https://www.producthunt.com/posts/cursor",
			Tagline: "AI code editor",
			Votes:   1234,
		},
	}
	got := normalizeProductHuntEdge(edge)
	if got.Title != "Cursor" || got.Source != "producthunt" {
		t.Fatalf("unexpected normalized item: %+v", got)
	}
}
```

- [ ] **Step 2: Run the new test and verify it fails**

Run: `go test ./internal/collector -run TestNormalizeProductHuntItem -v`

Expected: FAIL because `productHuntEdge`, `productHuntNode`, and `normalizeProductHuntEdge` do not exist yet.

- [ ] **Step 3: Implement the fetcher and pure helpers**

```go
type ProductHuntFetcher struct {
	APIToken string
}

func (p *ProductHuntFetcher) Name() string { return "producthunt" }

func (p *ProductHuntFetcher) Fetch() ([]NewsItem, error) {
	// 1. Try official GraphQL when APIToken is present.
	// 2. Fall back to public page scraping when token is missing or the API path fails.
	// 3. Normalize title, tagline, url, topics, votes, comments, makers, and rank into NewsItem.
}
```

Use these raw fields in `RawData`:

```go
RawData: map[string]any{
	"product_id":    node.ID,
	"slug":          node.Slug,
	"tagline":       node.Tagline,
	"topics":        topics,
	"votes_count":   votes,
	"comments_count": comments,
	"maker_names":   makers,
	"source":        "producthunt",
}
```

- [ ] **Step 4: Re-run collector tests**

Run: `go test ./internal/collector -v`

Expected: PASS.

- [ ] **Step 5: Verify the fetcher works against a live endpoint**

Run one of:

`PRODUCTHUNT_API_TOKEN=... go test ./internal/collector -run TestNormalizeProductHuntItem -v`

or a temporary one-off fetch command in a scratch `main` if network access is available.

## Task 3: Add Product Hunt storage routing and search filters

**Files:**
- Modify: `internal/storage/storage.go`
- Create: `internal/storage/storage_test.go`

- [ ] **Step 1: Write the failing storage integration test**

```go
func TestListNewsFiltersProductHuntByKeyword(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set")
	}

	store := mustNewTestStore(t, dsn)
	seedProductHuntNews(t, store)

	got, err := store.ListNews("producthunt", "hot", 20, "", "Cursor", "")
	if err != nil {
		t.Fatalf("ListNews error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
}
```

- [ ] **Step 2: Run the storage test and verify it fails**

Run: `TEST_POSTGRES_DSN=... go test ./internal/storage -run TestListNewsFiltersProductHuntByKeyword -v`

Expected: FAIL because `producthunt` routing and `q/tag` filtering are not wired yet.

- [ ] **Step 3: Extend schema routing and query building**

```go
allowedSources = []string{"github", "baidu", "gold", "ashare", "x", "hackernews", "producthunt"}
sourceToTable = map[string]string{
	"github": "news_github",
	"baidu": "news_baidu",
	"gold": "news_gold",
	"ashare": "news_ashare",
	"x": "news_x",
	"hackernews": "news_hackernews",
	"producthunt": "news_producthunt",
}
```

Update the `ListNews` signature and filter logic:

```go
func (s *Store) ListNews(channel, sort string, limit int, date, q, tag string) ([]News, error)
```

Query rules:

- `q` matches `title`, `description`, and the serialized topic list in `extra_data`.
- `tag` filters against the Product Hunt topic list only.
- empty `q` and `tag` keep the current behavior.

- [ ] **Step 4: Re-run storage tests**

Run: `go test ./internal/storage -v`

Expected: PASS.

- [ ] **Step 5: Check the default query paths still work**

Run: `go test ./...`

Expected: PASS, with `ListLatest` updated to call the new `ListNews` signature.

## Task 4: Expose search parameters through API and frontend

**Files:**
- Modify: `internal/api/router.go`
- Create: `internal/api/router_test.go`
- Modify: `web/src/api.ts`
- Modify: `web/src/types.ts`
- Modify: `web/src/App.tsx`
- Modify: `web/src/styles.css`

- [ ] **Step 1: Write the failing API request test or handler check**

Add a handler test that sends a request like this:

```go
req := httptest.NewRequest(http.MethodGet, "/api/v1/news?channel=producthunt&q=Cursor&tag=ai", nil)
```

Assert that the handler passes `q` and `tag` through to storage by using a tiny fake store with a `ListNews` recorder.

- [ ] **Step 2: Run the API test and verify it fails**

Run: `go test ./internal/api -v`

Expected: FAIL until `router.go` parses and forwards the new query params.

- [ ] **Step 3: Update the API contract**

```go
// internal/api/router.go
q := c.Query("q")
tag := c.Query("tag")
items, err := s.store.ListNews(channel, sort, limit, date, q, tag)
```

```ts
// web/src/api.ts
export async function fetchNews(params: {
  channel?: string;
  sort?: "latest" | "hot";
  limit?: number;
  date?: string;
  q?: string;
  tag?: string;
}): Promise<NewsItem[]> {
  if (params.q) search.set("q", params.q);
  if (params.tag) search.set("tag", params.tag);
}
```

- [ ] **Step 4: Add the Product Hunt UI controls**

```tsx
const [query, setQuery] = useState("");
const [tag, setTag] = useState("");

fetchNews({
  channel: "producthunt",
  sort: "hot",
  limit: 30,
  date: date || undefined,
  q: query || undefined,
  tag: tag || undefined,
});
```

UI requirements:

- Search input above the Product Hunt list.
- Tag chips for quick filtering.
- Clear button that resets `query` and `tag`.
- Keep the current card/list rendering style.

- [ ] **Step 5: Rebuild the frontend**

Run: `cd web && npm run build`

Expected: PASS.

## Task 5: Docs, end-to-end verification, and cleanup

**Files:**
- Modify: `README.md`
- Modify: any test files created above

- [ ] **Step 1: Update the README**

Add:

- Product Hunt as a feature channel
- `PRODUCTHUNT_API_TOKEN` if used
- the new `/api/v1/news` search parameters
- a short example showing `channel=producthunt&q=cursor`

- [ ] **Step 2: Run the full backend test suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Run the frontend build**

Run: `cd web && npm run build`

Expected: PASS.

- [ ] **Step 4: Manual smoke test**

Run:

```bash
curl "http://localhost:9000/api/v1/news?channel=producthunt&sort=hot&limit=10"
curl "http://localhost:9000/api/v1/news?channel=producthunt&q=cursor"
```

Expected:

- The response should be valid JSON.
- The returned items should only include Product Hunt results.
- Keyword filtering should narrow the list when data exists.

- [ ] **Step 5: Commit in one logical chunk**

```bash
git add cmd/api/main.go internal/config/config.go internal/config/config_test.go internal/collector/producthunt.go internal/collector/producthunt_test.go internal/storage/storage.go internal/storage/storage_test.go internal/api/router.go web/src/api.ts web/src/App.tsx web/src/types.ts web/src/styles.css README.md
git commit -m "feat: add Product Hunt search channel"
```

## Self-Review Checklist

- [ ] Every spec requirement has a task: sync, search, tag filtering, docs, verification.
- [ ] No placeholder text remains in the plan.
- [ ] The `ListNews` signature change is reflected everywhere that calls it.
- [ ] Product Hunt is isolated as its own source and does not disturb existing channels.
- [ ] The plan keeps the first implementation narrow: one new channel, one new search flow, no external search engine.


package collector

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const productHuntFeedURL = "https://www.producthunt.com/feed?category=undefined"
const productHuntGraphQLEndpoint = "https://api.producthunt.com/v2/api/graphql"

var (
	productHuntParagraphRe = regexp.MustCompile(`(?is)<p>(.*?)</p>`)
	productHuntTagRe       = regexp.MustCompile(`(?is)<[^>]+>`)
	productHuntIDRe        = regexp.MustCompile(`(?i)post/(\d+)`)
)

var productHuntTranslateText = TranslateToChinese

type productHuntGraphQLTopicNode struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type productHuntGraphQLTopicConnection struct {
	Nodes []productHuntGraphQLTopicNode `json:"nodes"`
}

type productHuntGraphQLMaker struct {
	Name string `json:"name"`
}

type productHuntGraphQLPost struct {
	ID            string                            `json:"id"`
	Name          string                            `json:"name"`
	Tagline       string                            `json:"tagline"`
	Slug          string                            `json:"slug"`
	URL           string                            `json:"url"`
	Website       string                            `json:"website"`
	CreatedAt     string                            `json:"createdAt"`
	FeaturedAt    string                            `json:"featuredAt"`
	VotesCount    int                               `json:"votesCount"`
	CommentsCount int                               `json:"commentsCount"`
	Makers        []productHuntGraphQLMaker         `json:"makers"`
	Topics        productHuntGraphQLTopicConnection `json:"topics"`
}

// ProductHuntFetcher 抓取 Product Hunt 公开 RSS feed
type ProductHuntFetcher struct {
	APIToken string
}

func (p *ProductHuntFetcher) Name() string {
	return "producthunt"
}

func (p *ProductHuntFetcher) Fetch() ([]NewsItem, error) {
	if strings.TrimSpace(p.APIToken) != "" {
		if items, err := p.fetchViaGraphQL(); err == nil && len(items) > 0 {
			return items, nil
		} else if err != nil {
			log.Printf("producthunt: graphql fetch failed, fallback to rss: %v", err)
		}
	}

	log.Println("fetch Product Hunt RSS...")

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, productHuntFeedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TrendingHubBot/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("producthunt: fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("producthunt: unexpected status %d", resp.StatusCode)
	}

	items, err := parseProductHuntAtom(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		log.Println("producthunt: no items fetched")
	}
	return items, nil
}

func (p *ProductHuntFetcher) fetchViaGraphQL() ([]NewsItem, error) {
	type postsEnvelope struct {
		Nodes []productHuntGraphQLPost `json:"nodes"`
	}
	type graphQLResponse struct {
		Data struct {
			Posts postsEnvelope `json:"posts"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	payload := map[string]any{
		"query": `
query ProductHuntPosts($first: Int!, $order: PostsOrder!) {
  posts(first: $first, order: $order) {
    nodes {
      id
      name
      tagline
      slug
      url
      website
      createdAt
      featuredAt
      votesCount
      commentsCount
      makers { name }
      topics(first: 5) { nodes { name slug } }
    }
  }
}`,
		"variables": map[string]any{
			"first": 30,
			"order": "VOTES",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, productHuntGraphQLEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TrendingHubBot/1.0")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(p.APIToken))

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("producthunt: graphql request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return nil, fmt.Errorf("producthunt: graphql status %d: %s", resp.StatusCode, strings.TrimSpace(string(bs)))
	}

	var res graphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("producthunt: graphql decode: %w", err)
	}
	if len(res.Errors) > 0 {
		return nil, fmt.Errorf("producthunt: graphql errors: %s", res.Errors[0].Message)
	}

	out := make([]NewsItem, 0, len(res.Data.Posts.Nodes))
	for _, post := range res.Data.Posts.Nodes {
		item, ok := normalizeProductHuntGraphQLPost(post)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

type productHuntFeed struct {
	Entries []productHuntEntry `xml:"entry"`
}

type productHuntEntry struct {
	ID        string            `xml:"id"`
	Published string            `xml:"published"`
	Updated   string            `xml:"updated"`
	Title     string            `xml:"title"`
	Content   string            `xml:"content"`
	Links     []productHuntLink `xml:"link"`
	Author    productHuntAuthor `xml:"author"`
}

type productHuntLink struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
	Href string `xml:"href,attr"`
}

type productHuntAuthor struct {
	Name string `xml:"name"`
}

func parseProductHuntAtom(r io.Reader) ([]NewsItem, error) {
	var feed productHuntFeed
	if err := xml.NewDecoder(r).Decode(&feed); err != nil {
		return nil, fmt.Errorf("producthunt: decode atom feed: %w", err)
	}

	out := make([]NewsItem, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		item, ok := normalizeProductHuntEntry(entry)
		if !ok {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func normalizeProductHuntEntry(entry productHuntEntry) (NewsItem, bool) {
	title := strings.TrimSpace(entry.Title)
	if title == "" {
		return NewsItem{}, false
	}

	itemURL := ""
	for _, link := range entry.Links {
		if strings.EqualFold(link.Rel, "alternate") && strings.TrimSpace(link.Href) != "" {
			itemURL = strings.TrimSpace(link.Href)
			break
		}
	}
	if itemURL == "" {
		return NewsItem{}, false
	}

	publishedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(entry.Published))
	if err != nil {
		if parsed, err2 := time.Parse(time.RFC3339, strings.TrimSpace(entry.Updated)); err2 == nil {
			publishedAt = parsed
		} else {
			publishedAt = time.Now()
		}
	}

	tagline := normalizeProductHuntDetailText(extractProductHuntTagline(entry.Content))
	if tagline == "" {
		tagline = normalizeProductHuntDetailText(title)
	}

	productID := extractProductHuntID(entry.ID)
	slug := extractProductHuntSlug(itemURL)
	author := strings.TrimSpace(entry.Author.Name)
	makerNames := []string{}
	if author != "" {
		makerNames = append(makerNames, author)
	}

	return NewsItem{
		Title:       title,
		URL:         itemURL,
		Source:      "producthunt",
		Description: tagline,
		PublishedAt: publishedAt,
		HotScore:    float64(publishedAt.Unix()),
		RawData: map[string]any{
			"product_id":     productID,
			"slug":           slug,
			"tagline":        tagline,
			"topics":         []string{},
			"votes_count":    0,
			"comments_count": 0,
			"maker_names":    makerNames,
			"author":         author,
			"source":         "producthunt",
		},
	}, true
}

func normalizeProductHuntGraphQLPost(post productHuntGraphQLPost) (NewsItem, bool) {
	title := strings.TrimSpace(post.Name)
	if title == "" {
		return NewsItem{}, false
	}
	itemURL := strings.TrimSpace(post.URL)
	if itemURL == "" {
		itemURL = strings.TrimSpace(post.Website)
	}
	if itemURL == "" {
		return NewsItem{}, false
	}

	publishedAt := time.Now()
	if ts := strings.TrimSpace(post.FeaturedAt); ts != "" {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			publishedAt = parsed
		}
	} else if ts := strings.TrimSpace(post.CreatedAt); ts != "" {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			publishedAt = parsed
		}
	}

	tagline := normalizeProductHuntDetailText(strings.TrimSpace(post.Tagline))
	if tagline == "" {
		tagline = normalizeProductHuntDetailText(title)
	}

	makerNames := make([]string, 0, len(post.Makers))
	for _, m := range post.Makers {
		name := strings.TrimSpace(m.Name)
		if name != "" {
			makerNames = append(makerNames, name)
		}
	}
	topics := make([]string, 0, len(post.Topics.Nodes))
	for _, topic := range post.Topics.Nodes {
		name := strings.TrimSpace(topic.Name)
		if name != "" {
			topics = append(topics, name)
			continue
		}
		slug := strings.TrimSpace(topic.Slug)
		if slug != "" {
			topics = append(topics, slug)
		}
	}

	return NewsItem{
		Title:       title,
		URL:         itemURL,
		Source:      "producthunt",
		Description: tagline,
		PublishedAt: publishedAt,
		HotScore:    float64(post.VotesCount),
		RawData: map[string]any{
			"product_id":     strings.TrimSpace(post.ID),
			"slug":           strings.TrimSpace(post.Slug),
			"tagline":        tagline,
			"topics":         topics,
			"votes_count":    post.VotesCount,
			"comments_count": post.CommentsCount,
			"maker_names":    makerNames,
			"source":         "producthunt",
		},
	}, true
}

func extractProductHuntTagline(content string) string {
	content = html.UnescapeString(strings.TrimSpace(content))
	if content == "" {
		return ""
	}

	matches := productHuntParagraphRe.FindAllStringSubmatch(content, -1)
	if len(matches) > 0 {
		tagline := stripProductHuntHTML(matches[0][1])
		return normalizeProductHuntWhitespace(tagline)
	}

	return normalizeProductHuntWhitespace(stripProductHuntHTML(content))
}

func normalizeProductHuntDetailText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if isMostlyChinese(text) {
		return text
	}
	return productHuntTranslateText(text)
}

func stripProductHuntHTML(s string) string {
	return productHuntTagRe.ReplaceAllString(s, " ")
}

func normalizeProductHuntWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func extractProductHuntSlug(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func extractProductHuntID(rawID string) string {
	m := productHuntIDRe.FindStringSubmatch(rawID)
	if len(m) != 2 {
		return ""
	}
	if _, err := strconv.Atoi(m[1]); err != nil {
		return ""
	}
	return m[1]
}

package collector

import (
	"strings"
	"testing"
	"time"
)

func TestProductHuntFetcherName(t *testing.T) {
	f := &ProductHuntFetcher{}
	if got := f.Name(); got != "producthunt" {
		t.Fatalf("Name() = %q, want %q", got, "producthunt")
	}
}

func TestParseProductHuntAtomEntry(t *testing.T) {
	const feed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>tag:www.producthunt.com,2005:Post/1140142</id>
    <published>2026-05-05T22:30:14-07:00</published>
    <updated>2026-05-11T19:23:26-07:00</updated>
    <link rel="alternate" type="text/html" href="https://www.producthunt.com/products/chatgpt-for-google-sheets"/>
    <title>ChatGPT for Google Sheets</title>
    <content type="html">&lt;p&gt;Chat with your spreadsheet, edit cell with natural language&lt;/p&gt;
&lt;p&gt;&lt;a href="https://www.producthunt.com/products/chatgpt-for-google-sheets"&gt;Discussion&lt;/a&gt; | &lt;a href="https://www.producthunt.com/r/p/1140142?app_id=339"&gt;Link&lt;/a&gt;&lt;/p&gt;</content>
    <author><name>Rohan Chaubey</name></author>
  </entry>
</feed>`

	got, err := parseProductHuntAtom(strings.NewReader(feed))
	if err != nil {
		t.Fatalf("parseProductHuntAtom error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	item := got[0]
	if item.Title != "ChatGPT for Google Sheets" {
		t.Fatalf("Title = %q, want %q", item.Title, "ChatGPT for Google Sheets")
	}
	if item.Source != "producthunt" {
		t.Fatalf("Source = %q, want %q", item.Source, "producthunt")
	}
	if item.URL != "https://www.producthunt.com/products/chatgpt-for-google-sheets" {
		t.Fatalf("URL = %q, want product URL", item.URL)
	}
	if item.Description != "Chat with your spreadsheet, edit cell with natural language" {
		t.Fatalf("Description = %q, want parsed tagline", item.Description)
	}
	if item.HotScore <= 0 {
		t.Fatalf("HotScore should be positive, got %v", item.HotScore)
	}
	if gotAt := item.PublishedAt.In(time.UTC).Format(time.RFC3339); gotAt == "" {
		t.Fatalf("PublishedAt should be set")
	}
	if item.RawData["author"] != "Rohan Chaubey" {
		t.Fatalf("author raw data = %#v, want %q", item.RawData["author"], "Rohan Chaubey")
	}
}

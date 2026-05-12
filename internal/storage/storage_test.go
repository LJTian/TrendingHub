package storage

import "testing"

func TestNewsTableIncludesProductHunt(t *testing.T) {
	if got := newsTable("producthunt"); got != "news_producthunt" {
		t.Fatalf("newsTable(producthunt) = %q, want %q", got, "news_producthunt")
	}
}

func TestNewsSearchClauseBuildsExpectedPattern(t *testing.T) {
	clause, args := newsSearchClause("Cursor")
	if clause == "" {
		t.Fatalf("newsSearchClause should return a non-empty clause")
	}
	if len(args) != 3 {
		t.Fatalf("newsSearchClause args = %d, want 3", len(args))
	}
	if args[0] != "%Cursor%" || args[1] != "%Cursor%" || args[2] != "%Cursor%" {
		t.Fatalf("unexpected search args: %#v", args)
	}
}

func TestNewsTagClauseBuildsExpectedPattern(t *testing.T) {
	clause, args := newsTagClause("ai")
	if clause == "" {
		t.Fatalf("newsTagClause should return a non-empty clause")
	}
	if len(args) != 1 || args[0] != "%ai%" {
		t.Fatalf("unexpected tag args: %#v", args)
	}
}

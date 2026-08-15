package main

import "testing"

func TestWorkspaceSearchMatches(t *testing.T) {
	content := "Alpha alpha\nalpha beta\n"

	matches := workspaceSearchMatches(content, "alpha", false, 10)
	if len(matches) != 3 {
		t.Fatalf("got %d matches, want 3", len(matches))
	}
	if matches[0].Line != 1 || matches[0].Column != 1 {
		t.Fatalf("first match at %d:%d, want 1:1", matches[0].Line, matches[0].Column)
	}

	caseSensitive := workspaceSearchMatches(content, "alpha", true, 10)
	if len(caseSensitive) != 2 {
		t.Fatalf("got %d case-sensitive matches, want 2", len(caseSensitive))
	}
}

func TestWorkspaceReplaceAll(t *testing.T) {
	updated, count := workspaceReplaceAll("Alpha alpha ALPHA", "alpha", "beta", false)
	if count != 3 {
		t.Fatalf("got %d replacements, want 3", count)
	}
	if updated != "beta beta beta" {
		t.Fatalf("got %q, want %q", updated, "beta beta beta")
	}

	updated, count = workspaceReplaceAll("Alpha alpha ALPHA", "alpha", "beta", true)
	if count != 1 || updated != "Alpha beta ALPHA" {
		t.Fatalf("case-sensitive replacement got (%q, %d)", updated, count)
	}
}

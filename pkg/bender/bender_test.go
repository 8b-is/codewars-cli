package bender

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8b-is/codewars-cli/pkg/codewars"
)

func TestScaffoldAndDiscover(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "katas"), 0o755); err != nil {
		t.Fatal(err)
	}

	rankID := -2
	rankName := "2 kyu"
	rankColor := "purple"
	ch := &codewars.CodeChallenge{
		ID:          "52a78825cdfc2cfc87000005",
		Name:        "Evaluate mathematical expression",
		Slug:        "evaluate-mathematical-expression",
		URL:         "https://www.codewars.com/kata/52a78825cdfc2cfc87000005",
		Description: "Given a mathematical expression as a string you must return the result as a number.",
		Rank: &codewars.ChallengeRank{
			ID:    &rankID,
			Name:  &rankName,
			Color: &rankColor,
		},
	}

	dir, err := ScaffoldKata(tmp, ch, "python", "evaluate-math")
	if err != nil {
		t.Fatalf("ScaffoldKata: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
		t.Fatalf("missing README.md: %v", err)
	}

	entries, err := DiscoverKatas(tmp)
	if err != nil {
		t.Fatalf("DiscoverKatas: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 kata, got %d", len(entries))
	}
	e := entries[0]
	if e.Tier != "2kyu" || e.Slug != "evaluate-math" || e.Language != "python" || e.CodewarsID != "52a78825cdfc2cfc87000005" {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

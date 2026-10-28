package bender

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/8b-is/codewars-cli/pkg/codewars"
)

const fibCompletedJSON = `{
  "totalPages": 2,
  "totalItems": 281,
  "data": [
    {"id":"57a1d5ef7cb1f3db590002af","name":"Fibonacci","slug":"fibonacci",
     "completedAt":"2026-10-11T12:00:00.000Z","completedLanguages":["lambdacalc"]},
    {"id":"deadbeefdeadbeefdeadbeef","name":"Not Local","slug":"not-local",
     "completedAt":"2026-10-12T12:00:00.000Z","completedLanguages":["python"]}
  ]
}`

// scaffoldRepo builds a throwaway space-bender-like tree with one kata.
func scaffoldRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	kataDir := filepath.Join(dir, "katas", "7kyu", "fibonacci")
	if err := os.MkdirAll(kataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kataDir, "README.md"),
		[]byte("# Fibonacci (λ Calculus)\n\n**Kyu:** 7 · **Source:** [Codewars](https://www.codewars.com/kata/57a1d5ef7cb1f3db590002af)\n\n## Verified\n\nok locally.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kataDir, "solution.lc"), []byte("id = \\x . x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func testClient(t *testing.T) (*codewars.Client, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/code-challenges/completed") {
			hits++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fibCompletedJSON))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	c := codewars.NewClient()
	c.BaseURL = srv.URL
	return c, &hits
}

func TestRecordCompletionsWritesLedgerAndReadme(t *testing.T) {
	repo := scaffoldRepo(t)
	client, _ := testClient(t)

	res, err := RecordCompletions(context.Background(), client, repo, RecordOptions{User: "szepnapot"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed != 1 || len(res.Recorded) != 1 || res.Recorded[0].Slug != "fibonacci" {
		t.Fatalf("expected 1 recorded fibonacci, got %+v", res)
	}

	ledger, err := os.ReadFile(filepath.Join(repo, LEDGERFile))
	if err != nil {
		t.Fatalf("ledger missing: %v", err)
	}
	if !strings.Contains(string(ledger), "`57a1d5ef7cb1f3db590002af`") {
		t.Fatalf("ledger lacks kata id:\n%s", ledger)
	}

	readme, err := os.ReadFile(filepath.Join(repo, "katas", "7kyu", "fibonacci", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "**Platform:** PASS") {
		t.Fatalf("readme lacks platform line:\n%s", readme)
	}
}

func TestRecordCompletionsIsIdempotent(t *testing.T) {
	repo := scaffoldRepo(t)
	client, _ := testClient(t)

	if _, err := RecordCompletions(context.Background(), client, repo, RecordOptions{User: "szepnapot"}); err != nil {
		t.Fatal(err)
	}
	res, err := RecordCompletions(context.Background(), client, repo, RecordOptions{User: "szepnapot"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed != 0 {
		t.Fatalf("second pass changed %d rows, want 0", res.Changed)
	}
	if len(res.Unchanged) != 1 || res.Unchanged[0].Action != "already-recorded" {
		t.Fatalf("expected the fibonacci row to be already-recorded, got %+v", res.Unchanged)
	}
	ledger, _ := os.ReadFile(filepath.Join(repo, LEDGERFile))
	if got := strings.Count(string(ledger), "`57a1d5ef7cb1f3db590002af`"); got != 1 {
		t.Fatalf("ledger has %d rows for the kata, want 1", got)
	}
	readme, _ := os.ReadFile(filepath.Join(repo, "katas", "7kyu", "fibonacci", "README.md"))
	if got := strings.Count(string(readme), "**Platform:**"); got != 1 {
		t.Fatalf("readme has %d platform lines, want 1", got)
	}
}

func TestRecordCompletionsDryRunWritesNothing(t *testing.T) {
	repo := scaffoldRepo(t)
	client, _ := testClient(t)

	res, err := RecordCompletions(context.Background(), client, repo, RecordOptions{User: "szepnapot", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Recorded) != 1 {
		t.Fatalf("dry run should still report the candidate, got %+v", res)
	}
	if _, err := os.Stat(filepath.Join(repo, LEDGERFile)); !os.IsNotExist(err) {
		t.Fatalf("dry run created a ledger")
	}
	readme, _ := os.ReadFile(filepath.Join(repo, "katas", "7kyu", "fibonacci", "README.md"))
	if strings.Contains(string(readme), "**Platform:**") {
		t.Fatalf("dry run modified the readme")
	}
}

func TestRecordCompletionsCommits(t *testing.T) {
	repo := scaffoldRepo(t)
	client, _ := testClient(t)

	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "hook@ombi.is"},
		{"config", "user.name", "hookbot"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}

	res, err := RecordCompletions(context.Background(), client, repo, RecordOptions{User: "szepnapot", Commit: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.CommitSHA == "" {
		t.Fatal("expected a commit sha")
	}
	logCmd := exec.Command("git", "log", "--oneline", "-1")
	logCmd.Dir = repo
	out, err := logCmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "fibonacci") || !strings.Contains(string(out), "szepnapot") {
		t.Fatalf("unexpected commit message: %s", out)
	}
}

func TestLedgerJSONShape(t *testing.T) {
	repo := scaffoldRepo(t)
	client, _ := testClient(t)
	res, err := RecordCompletions(context.Background(), client, repo, RecordOptions{User: "szepnapot", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := json.Marshal(res); err != nil {
		t.Fatalf("result must be json-serialisable: %v", err)
	}
}

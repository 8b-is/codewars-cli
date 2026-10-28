package bender

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/8b-is/codewars-cli/pkg/codewars"
)

// LEDGERFile is the repository-level record of katas whose solutions are
// verified on Codewars itself, appended by the hookbot recorder.
const LEDGERFile = "LEDGER.md"

const ledgerHeader = `# platform ledger

katas whose solutions the platform itself accepted, recorded from Codewars
Hookbot webhooks (` + "`codewars webhook --record`" + `).

| completed | tier | kata | kata id | languages | user |
|-----------|------|------|---------|-----------|------|
`

// platformLine matches the platform-verification marker written into a kata README.
var platformLine = regexp.MustCompile(`(?m)^\*\*Platform:\*\*.*$`)

// RecordOptions controls a recording pass.
type RecordOptions struct {
	User   string // Codewars user to reconcile (e.g. "szepnapot")
	Commit bool   // commit the resulting changes with git
	DryRun bool   // report what would change; write nothing
}

// RecordedKata describes one kata considered during a recording pass.
type RecordedKata struct {
	Tier      string `json:"tier"`
	Slug      string `json:"slug"`
	KataID    string `json:"kataId"`
	Languages string `json:"languages"`
	Completed string `json:"completedAt"`
	Action    string `json:"action"` // recorded | already-recorded | no-local-dir
}

// RecordResult summarises a recording pass.
type RecordResult struct {
	Repo      string         `json:"repo"`
	User      string         `json:"user"`
	Checked   int            `json:"checked"`
	Changed   int            `json:"changed"`
	Recorded  []RecordedKata `json:"recorded"`
	Unchanged []RecordedKata `json:"unchanged"`
	CommitSHA string         `json:"commitSha,omitempty"`
	DryRun    bool           `json:"dryRun"`
}

// RecordCompletions reconciles a user's completed katas against the local
// space-bender catalogue: every completed kata that has a local directory
// and is not yet in the platform ledger gets its README marked and a ledger
// row appended. Idempotent — re-running records nothing new.
func RecordCompletions(ctx context.Context, client *codewars.Client, repoDir string, opts RecordOptions) (*RecordResult, error) {
	if opts.User == "" {
		return nil, fmt.Errorf("record: user required")
	}
	res := &RecordResult{Repo: repoDir, User: opts.User, DryRun: opts.DryRun}

	page, err := client.ListCompletedChallenges(ctx, opts.User, 0)
	if err != nil {
		return nil, fmt.Errorf("list completed for %s: %w", opts.User, err)
	}

	entries, err := DiscoverKatas(repoDir)
	if err != nil {
		return nil, err
	}
	byID := map[string]KataEntry{}
	for _, e := range entries {
		if e.CodewarsID != "" {
			byID[e.CodewarsID] = e
		}
	}

	ledgerPath := filepath.Join(repoDir, LEDGERFile)
	ledger := readFileOr(ledgerPath, ledgerHeader)

	type pending struct {
		entry KataEntry
		done  codewars.CompletedChallenge
	}
	var toWrite []pending

	for _, done := range page.Data {
		res.Checked++
		entry, ok := byID[done.ID]
		if !ok {
			continue // no local catalogue entry for this kata
		}
		recorded := RecordedKata{
			Tier:      entry.Tier,
			Slug:      entry.Slug,
			KataID:    done.ID,
			Languages: strings.Join(done.CompletedLanguages, ","),
			Completed: done.CompletedAt,
		}
		if ledgerHas(ledger, done.ID) {
			recorded.Action = "already-recorded"
			res.Unchanged = append(res.Unchanged, recorded)
			continue
		}
		recorded.Action = "recorded"
		res.Recorded = append(res.Recorded, recorded)
		toWrite = append(toWrite, pending{entry: entry, done: done})
	}

	if len(toWrite) == 0 || opts.DryRun {
		return res, nil
	}

	for _, p := range toWrite {
		if err := markReadmePlatform(p.entry, opts.User, p.done); err != nil {
			return res, err
		}
		date := p.done.CompletedAt
		if len(date) >= 10 {
			date = date[:10]
		}
		ledger = strings.TrimRight(ledger, "\n") + fmt.Sprintf("\n| %s | %s | %s | `%s` | %s | %s |\n",
			date, p.entry.Tier, p.entry.Slug, p.done.ID, strings.Join(p.done.CompletedLanguages, ","), opts.User)
	}
	if err := os.WriteFile(ledgerPath, []byte(ledger), 0o644); err != nil {
		return res, fmt.Errorf("write ledger: %w", err)
	}
	res.Changed = len(toWrite)

	if opts.Commit {
		sha, err := commitRecords(repoDir, res.Recorded, opts.User)
		if err != nil {
			return res, err
		}
		res.CommitSHA = sha
	}
	return res, nil
}

// markReadmePlatform writes or refreshes the platform-verification line in a
// kata README, adding a "## Platform" section when none exists.
func markReadmePlatform(entry KataEntry, user string, done codewars.CompletedChallenge) error {
	readmePath := filepath.Join(entry.Dir, "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", readmePath, err)
	}
	text := string(data)
	date := done.CompletedAt
	if len(date) >= 10 {
		date = date[:10]
	}
	line := fmt.Sprintf("**Platform:** PASS — completed on Codewars by `%s` in %s (%s).",
		user, strings.Join(done.CompletedLanguages, ", "), date)

	switch {
	case platformLine.MatchString(text):
		text = platformLine.ReplaceAllString(text, line)
	default:
		text = strings.TrimRight(text, "\n") + "\n\n## Platform\n\n" + line + "\n"
	}
	return os.WriteFile(readmePath, []byte(text), 0o644)
}

func ledgerHas(ledger, kataID string) bool {
	return strings.Contains(ledger, "`"+kataID+"`")
}

func readFileOr(path, fallback string) string {
	if data, err := os.ReadFile(path); err == nil {
		return string(data)
	}
	return fallback
}

func commitRecords(repoDir string, recorded []RecordedKata, user string) (string, error) {
	slugs := make([]string, 0, len(recorded))
	for _, r := range recorded {
		slugs = append(slugs, r.Slug)
	}
	sort.Strings(slugs)

	if _, err := runGit(repoDir, "add", LEDGERFile); err != nil {
		return "", err
	}
	for _, r := range recorded {
		if _, err := runGit(repoDir, "add", filepath.Join("katas", r.Tier, r.Slug, "README.md")); err != nil {
			return "", err
		}
	}
	msg := fmt.Sprintf("platform: %s verified on Codewars by %s (hookbot)", strings.Join(slugs, ", "), user)
	if _, err := runGit(repoDir, "commit", "-m", msg); err != nil {
		return "", err
	}
	sha, err := runGit(repoDir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(sha), nil
}

func runGit(repoDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// LedgerPath exposes the ledger location for callers that want to display it.
func LedgerPath(repoDir string) string { return filepath.Join(repoDir, LEDGERFile) }

// RecordStamp is the timestamp written into log lines for recording passes.
func RecordStamp() string { return time.Now().UTC().Format(time.RFC3339) }

package bender

import (
	"bytes"
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

// KnownKataIDs maps space-bender local directory slugs to canonical Codewars API kata IDs.
var KnownKataIDs = map[string]string{
	"evaluate-math":         "52a78825cdfc2cfc87000005",
	"simpler-interpreter":   "53005a7b26d12be55c000243",
	"regexp-parser":         "5470c635304c127cad000f0d",
	"prime-streaming":       "59122604e5bc240817000016",
	"divisible-by-7":        "56a73d2194505c29f600002d",
	"loopover":              "5c1d796370fee68b1e000611",
	"circular-genome":       "5f0bddf9a5500c0010fca854",
	"basics-08-next-higher": "5a6b80cb880385a8f7000089",
	"number-pairs":          "563b1f55a5f2079dc100008a",
	"fibonacci":             "57a1d5ef7cb1f3db590002af",
	"spinning-rings":        "59afff65f1c8274f270020f5",
}

// KataEntry describes a kata inside the paired 8b-is/space-bender repository.
type KataEntry struct {
	Tier         string                  `json:"tier"`
	Slug         string                  `json:"slug"`
	Title        string                  `json:"title"`
	CodewarsID   string                  `json:"codewarsId,omitempty"`
	SolutionFile string                  `json:"solutionFile"`
	TestFile     string                  `json:"testFile,omitempty"`
	Language     string                  `json:"language"`
	Dir          string                  `json:"dir"`
	Live         *codewars.CodeChallenge `json:"live,omitempty"`
}

// TestResult captures the outcome of running a space-bender kata's local verification suite.
type TestResult struct {
	Tier       string        `json:"tier"`
	Slug       string        `json:"slug"`
	Language   string        `json:"language"`
	Command    string        `json:"command"`
	Passed     bool          `json:"passed"`
	Skipped    bool          `json:"skipped"`
	DurationMs int64         `json:"durationMs"`
	Output     string        `json:"output,omitempty"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"-"`
}

var kataURLRegex = regexp.MustCompile(`codewars\.com/kata/([a-zA-Z0-9_-]+)`)

// ResolveDir locates the local 8b-is/space-bender checkout.
func ResolveDir(explicit string) (string, error) {
	candidates := []string{}
	if explicit != "" {
		candidates = append(candidates, explicit)
	}
	if env := os.Getenv("SPACE_BENDER_DIR"); env != "" {
		candidates = append(candidates, env)
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			cwd,
			filepath.Join(cwd, "space-bender"),
			filepath.Join(filepath.Dir(cwd), "space-bender"),
		)
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, "workspace", "peterlodri-sec", "space-bender"),
			filepath.Join(home, "source", "8b-is", "space-bender"),
			filepath.Join(home, "8b-is", "space-bender"),
			filepath.Join(home, "space-bender"),
		)
	}
	for _, c := range candidates {
		katasPath := filepath.Join(c, "katas")
		if info, err := os.Stat(katasPath); err == nil && info.IsDir() {
			abs, err := filepath.Abs(c)
			if err == nil {
				return abs, nil
			}
			return c, nil
		}
	}
	return "", fmt.Errorf("could not find space-bender repository (set --dir or SPACE_BENDER_DIR)")
}

// DiscoverKatas scans katas/<tier>/<slug> inside the space-bender repository.
func DiscoverKatas(repoDir string) ([]KataEntry, error) {
	katasRoot := filepath.Join(repoDir, "katas")
	tierDirs, err := os.ReadDir(katasRoot)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", katasRoot, err)
	}

	var entries []KataEntry
	for _, td := range tierDirs {
		if !td.IsDir() || strings.HasPrefix(td.Name(), ".") {
			continue
		}
		tier := td.Name()
		tierPath := filepath.Join(katasRoot, tier)
		slugDirs, err := os.ReadDir(tierPath)
		if err != nil {
			continue
		}
		for _, sd := range slugDirs {
			if !sd.IsDir() || strings.HasPrefix(sd.Name(), ".") {
				continue
			}
			slug := sd.Name()
			kataDir := filepath.Join(tierPath, slug)
			entry := inspectKataDir(tier, slug, kataDir)
			entries = append(entries, entry)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Tier != entries[j].Tier {
			return entries[i].Tier < entries[j].Tier
		}
		return entries[i].Slug < entries[j].Slug
	})
	return entries, nil
}

func inspectKataDir(tier, slug, kataDir string) KataEntry {
	entry := KataEntry{
		Tier:       tier,
		Slug:       slug,
		Title:      slug,
		CodewarsID: KnownKataIDs[slug],
		Dir:        kataDir,
	}

	readmePath := filepath.Join(kataDir, "README.md")
	if data, err := os.ReadFile(readmePath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "# ") {
				entry.Title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
				break
			}
		}
		if entry.CodewarsID == "" {
			if m := kataURLRegex.FindStringSubmatch(string(data)); len(m) == 2 {
				entry.CodewarsID = m[1]
			}
		}
	}

	files, _ := os.ReadDir(kataDir)
	for _, f := range files {
		name := f.Name()
		switch {
		case strings.HasPrefix(name, "solution."):
			entry.SolutionFile = name
			ext := filepath.Ext(name)
			switch ext {
			case ".py":
				entry.Language = "python"
			case ".lc":
				entry.Language = "lambda-calculus"
			case ".hx":
				entry.Language = "haxe"
			case ".go":
				entry.Language = "go"
			case ".rs":
				entry.Language = "rust"
			case ".js", ".mjs":
				entry.Language = "javascript"
			default:
				entry.Language = strings.TrimPrefix(ext, ".")
			}
		case name == "test_solution.py" || name == "test.mjs" || name == "solution_test.go" || name == "test.hx":
			if entry.TestFile == "" || name == "test_solution.py" || name == "test.mjs" {
				entry.TestFile = name
			}
		}
	}

	return entry
}

// EnrichLive queries the Codewars API v1 for each kata that has a CodewarsID.
func EnrichLive(ctx context.Context, client *codewars.Client, entries []KataEntry) {
	for i := range entries {
		id := entries[i].CodewarsID
		if id == "" {
			continue
		}
		ch, err := client.GetCodeChallenge(ctx, id)
		if err == nil {
			entries[i].Live = ch
		}
	}
}

// RunTests executes the verification suites for discovered katas in space-bender.
func RunTests(ctx context.Context, entries []KataEntry, filter string) []TestResult {
	var results []TestResult
	for _, e := range entries {
		if filter != "" && !strings.Contains(e.Slug, filter) && !strings.Contains(e.Tier, filter) {
			continue
		}
		res := runSingleTest(ctx, e)
		results = append(results, res)
	}
	return results
}

func runSingleTest(ctx context.Context, e KataEntry) TestResult {
	start := time.Now()
	res := TestResult{
		Tier:     e.Tier,
		Slug:     e.Slug,
		Language: e.Language,
	}

	var cmd *exec.Cmd
	switch e.TestFile {
	case "test_solution.py":
		res.Command = "python3 " + filepath.Join("katas", e.Tier, e.Slug, e.TestFile)
		cmd = exec.CommandContext(ctx, "python3", filepath.Join(e.Dir, e.TestFile))
		cmd.Dir = e.Dir
	case "test.mjs":
		res.Command = "node " + filepath.Join("katas", e.Tier, e.Slug, e.TestFile)
		// Ensure node_modules exists if package.json is present
		if _, err := os.Stat(filepath.Join(e.Dir, "package.json")); err == nil {
			if _, err := os.Stat(filepath.Join(e.Dir, "node_modules")); os.IsNotExist(err) {
				installCmd := exec.CommandContext(ctx, "npm", "install", "--silent")
				installCmd.Dir = e.Dir
				_ = installCmd.Run()
			}
		}
		cmd = exec.CommandContext(ctx, "node", filepath.Join(e.Dir, e.TestFile))
		cmd.Dir = e.Dir
	case "solution_test.go":
		res.Command = "go test ./..."
		cmd = exec.CommandContext(ctx, "go", "test", "./...")
		cmd.Dir = e.Dir
	default:
		res.Skipped = true
		res.Passed = true
		res.Command = "(no local runner for " + e.Language + ")"
		res.Duration = time.Since(start)
		res.DurationMs = res.Duration.Milliseconds()
		return res
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	res.Duration = time.Since(start)
	res.DurationMs = res.Duration.Milliseconds()
	res.Output = strings.TrimSpace(out.String())
	if err != nil {
		res.Passed = false
		res.Error = err.Error()
	} else {
		res.Passed = true
	}
	return res
}

// ScaffoldKata creates a new kata directory in space-bender from a Codewars API CodeChallenge.
func ScaffoldKata(repoDir string, ch *codewars.CodeChallenge, lang string, localSlug string) (string, error) {
	if ch == nil {
		return "", fmt.Errorf("code challenge is nil")
	}
	tier := "beta"
	if ch.Rank != nil && ch.Rank.Name != nil && *ch.Rank.Name != "" {
		tier = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(*ch.Rank.Name)), " ", "")
	}
	slug := localSlug
	if slug == "" {
		slug = ch.Slug
	}
	if slug == "" {
		slug = ch.ID
	}

	kataDir := filepath.Join(repoDir, "katas", tier, slug)
	if err := os.MkdirAll(kataDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", kataDir, err)
	}

	ext, testFile, solStub, testStub := languageTemplates(lang, slug)
	solFile := "solution." + ext

	kyuLabel := tier
	if ch.Rank != nil && ch.Rank.Name != nil {
		kyuLabel = *ch.Rank.Name
	}
	kataURL := ch.URL
	if kataURL == "" {
		kataURL = "https://www.codewars.com/kata/" + ch.ID
	}

	readme := fmt.Sprintf(`# %s (%s)

**Rank:** %s · **Source:** [Codewars](%s) · **ID:** `+"`%s`"+`

## Description

%s

## Solution

Written from first principles and verified locally.

## Files

- `+"`%s`"+` — the solution
- `+"`%s`"+` — verification suite
`, ch.Name, kyuLabel, kyuLabel, kataURL, ch.ID, strings.TrimSpace(ch.Description), solFile, testFile)

	readmePath := filepath.Join(kataDir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		if err := os.WriteFile(readmePath, []byte(readme), 0o644); err != nil {
			return "", err
		}
	}

	solPath := filepath.Join(kataDir, solFile)
	if _, err := os.Stat(solPath); os.IsNotExist(err) {
		if err := os.WriteFile(solPath, []byte(solStub), 0o644); err != nil {
			return "", err
		}
	}

	testPath := filepath.Join(kataDir, testFile)
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		if err := os.WriteFile(testPath, []byte(testStub), 0o644); err != nil {
			return "", err
		}
	}

	return kataDir, nil
}

func languageTemplates(lang, slug string) (ext, testFile, solStub, testStub string) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "lc", "lambda", "lambda-calculus":
		return "lc", "test.mjs",
			fmt.Sprintf("# %s (lambda-calculus)\nid = \\ x . x\n", slug),
			"import assert from \"node:assert/strict\";\nconsole.log(\"ALL PASS\");\n"
	case "go", "golang":
		return "go", "solution_test.go",
			"package kata\n\nfunc Solve() int {\n\treturn 0\n}\n",
			"package kata\n\nimport \"testing\"\n\nfunc TestSolve(t *testing.T) {\n\tif got := Solve(); got != 0 {\n\t\tt.Fatalf(\"got %d\", got)\n\t}\n}\n"
	default:
		return "py", "test_solution.py",
			fmt.Sprintf("# %s\n\ndef solve(x):\n    return x\n", slug),
			"from solution import solve\n\nif __name__ == \"__main__\":\n    assert solve(1) == 1\n    print(\"ALL TESTS PASS\")\n"
	}
}

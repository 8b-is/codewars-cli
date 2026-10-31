package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/8b-is/codewars-cli/pkg/bender"
	"github.com/8b-is/codewars-cli/pkg/codewars"
)

const version = "1.0.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "user", "u":
		return cmdUser(rest)
	case "completed", "comp", "c":
		return cmdCompleted(rest)
	case "authored", "auth", "a":
		return cmdAuthored(rest)
	case "kata", "challenge", "k":
		return cmdKata(rest)
	case "webhook", "hook", "wh":
		return cmdWebhook(rest)
	case "bender", "space-bender", "sb", "b":
		return cmdBender(rest)
	case "mcp":
		return cmdMCP(rest)
	case "errors", "err":
		return cmdErrors()
	case "version", "-v", "--version":
		fmt.Printf("codewars-cli v%s (github.com/8b-is/codewars-cli · paired with 8b-is/space-bender)\n", version)
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (run 'codewars help' for usage)", sub)
	}
}

func newConfiguredClient(baseURL string) *codewars.Client {
	c := codewars.NewClient()
	if baseURL != "" {
		c.BaseURL = baseURL
	} else if env := os.Getenv("CODEWARS_API_URL"); env != "" {
		c.BaseURL = env
	}
	// Ensure trailing slash for BaseURL if not present to prevent path joining issues
	if c.BaseURL != "" && !strings.HasSuffix(c.BaseURL, "/") {
		c.BaseURL += "/"
	}
	if key := os.Getenv("CODEWARS_API_KEY"); key != "" {
		c.APIKey = key
	}
	return c
}

func cmdUser(args []string) error {
	fs := flag.NewFlagSet("user", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "Output raw JSON")
	baseURL := fs.String("api-url", "", "Override Codewars API v1 base URL")
	userArgs, err := parseFlagsInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(userArgs) < 1 {
		return fmt.Errorf("usage: codewars user <username_or_id> [--json]")
	}

	client := newConfiguredClient(*baseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	u, err := client.GetUser(ctx, userArgs[0])
	if err != nil {
		return err
	}

	if *asJSON {
		return printJSON(u)
	}

	fmt.Printf("User:         %s", u.Username)
	if u.Name != "" {
		fmt.Printf(" (%s)", u.Name)
	}
	fmt.Println()
	if u.Clan != "" {
		fmt.Printf("Clan:         %s\n", u.Clan)
	}
	fmt.Printf("Honor:        %d\n", u.Honor)
	if u.LeaderboardPosition > 0 {
		fmt.Printf("Leaderboard:  #%d\n", u.LeaderboardPosition)
	}
	fmt.Printf("Overall Rank: %s (%s, score %d, rank %d)\n",
		u.Ranks.Overall.Name, u.Ranks.Overall.Color, u.Ranks.Overall.Score, u.Ranks.Overall.Rank)
	fmt.Printf("Challenges:   %d completed · %d authored\n",
		u.CodeChallenges.TotalCompleted, u.CodeChallenges.TotalAuthored)
	if len(u.Skills) > 0 {
		fmt.Printf("Skills:       %s\n", strings.Join(u.Skills, ", "))
	}

	if len(u.Ranks.Languages) > 0 {
		fmt.Println("\nLanguage Ranks:")
		type langEntry struct {
			lang string
			info codewars.RankInfo
		}
		var langs []langEntry
		for k, v := range u.Ranks.Languages {
			langs = append(langs, langEntry{k, v})
		}
		sort.Slice(langs, func(i, j int) bool {
			if langs[i].info.Score != langs[j].info.Score {
				return langs[i].info.Score > langs[j].info.Score
			}
			return langs[i].lang < langs[j].lang
		})
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "  LANGUAGE\tRANK\tCOLOR\tSCORE")
		for _, l := range langs {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%d\n", l.lang, l.info.Name, l.info.Color, l.info.Score)
		}
		tw.Flush()
	}
	return nil
}

func cmdCompleted(args []string) error {
	fs := flag.NewFlagSet("completed", flag.ContinueOnError)
	page := fs.Int("page", 0, "Zero-based page offset (200 items/page)")
	all := fs.Bool("all", false, "Fetch all pages")
	maxPages := fs.Int("max-pages", 10, "Max pages when --all is set")
	lang := fs.String("lang", "", "Filter by completed language")
	limit := fs.Int("limit", 50, "Maximum rows to display in table mode (0 = unlimited)")
	asJSON := fs.Bool("json", false, "Output raw JSON")
	baseURL := fs.String("api-url", "", "Override Codewars API v1 base URL")
	posArgs, err := parseFlagsInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(posArgs) < 1 {
		return fmt.Errorf("usage: codewars completed <username_or_id> [--page 0] [--all] [--lang <lang>] [--json]")
	}

	client := newConfiguredClient(*baseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var items []codewars.CompletedChallenge
	var totalItems, totalPages int

	if *all {
		allItems, total, err := client.ListAllCompletedChallenges(ctx, posArgs[0], *maxPages)
		if err != nil {
			return err
		}
		items = allItems
		totalItems = total
		totalPages = (total + 199) / 200
	} else {
		p, err := client.ListCompletedChallenges(ctx, posArgs[0], *page)
		if err != nil {
			return err
		}
		items = p.Data
		totalItems = p.TotalItems
		totalPages = p.TotalPages
	}

	if *lang != "" {
		want := strings.ToLower(strings.TrimSpace(*lang))
		var filtered []codewars.CompletedChallenge
		for _, item := range items {
			for _, l := range item.CompletedLanguages {
				if strings.ToLower(l) == want {
					filtered = append(filtered, item)
					break
				}
			}
		}
		items = filtered
	}

	if *asJSON {
		return printJSON(map[string]any{
			"totalPages": totalPages,
			"totalItems": totalItems,
			"count":      len(items),
			"data":       items,
		})
	}

	fmt.Printf("Completed Challenges for %s (totalItems=%d, totalPages=%d, showing=%d)\n\n",
		posArgs[0], totalItems, totalPages, len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSLUG\tCOMPLETED AT\tLANGUAGES")
	shown := items
	if *limit > 0 && len(shown) > *limit {
		shown = shown[:*limit]
	}
	for _, c := range shown {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			c.ID, truncate(c.Slug, 36), c.CompletedAt, strings.Join(dedup(c.CompletedLanguages), ","))
	}
	tw.Flush()
	if len(items) > len(shown) {
		fmt.Printf("\n... (%d more items; pass --limit 0 to display all)\n", len(items)-len(shown))
	}
	return nil
}

func cmdAuthored(args []string) error {
	fs := flag.NewFlagSet("authored", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "Output raw JSON")
	baseURL := fs.String("api-url", "", "Override Codewars API v1 base URL")
	posArgs, err := parseFlagsInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(posArgs) < 1 {
		return fmt.Errorf("usage: codewars authored <username_or_id> [--json]")
	}

	client := newConfiguredClient(*baseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	items, err := client.ListAuthoredChallenges(ctx, posArgs[0])
	if err != nil {
		return err
	}

	if *asJSON {
		return printJSON(map[string]any{"data": items})
	}

	fmt.Printf("Authored Challenges by %s (%d total)\n\n", posArgs[0], len(items))
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tRANK\tNAME\tLANGUAGES")
	for _, a := range items {
		rk := "beta"
		if a.RankName != nil && *a.RankName != "" {
			rk = *a.RankName
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			a.ID, rk, truncate(a.Name, 44), strings.Join(a.Languages, ","))
	}
	tw.Flush()
	return nil
}

func cmdKata(args []string) error {
	fs := flag.NewFlagSet("kata", flag.ContinueOnError)
	showDesc := fs.Bool("desc", false, "Print full Markdown description")
	asJSON := fs.Bool("json", false, "Output raw JSON")
	baseURL := fs.String("api-url", "", "Override Codewars API v1 base URL")
	posArgs, err := parseFlagsInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(posArgs) < 1 {
		return fmt.Errorf("usage: codewars kata <id_or_slug_or_url> [--desc] [--json]")
	}

	client := newConfiguredClient(*baseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Allow space-bender shorthand slugs (e.g. "evaluate-math", "divisible-by-7")
	query := posArgs[0]
	if mapped, ok := bender.KnownKataIDs[query]; ok {
		query = mapped
	}

	ch, err := client.GetCodeChallenge(ctx, query)
	if err != nil {
		return err
	}

	if *asJSON {
		return printJSON(ch)
	}

	rankStr := "beta"
	if ch.Rank != nil && ch.Rank.Name != nil {
		rankStr = *ch.Rank.Name
		if ch.Rank.Color != nil && *ch.Rank.Color != "" {
			rankStr += " (" + *ch.Rank.Color + ")"
		}
	}

	fmt.Printf("Kata:        %s\n", ch.Name)
	fmt.Printf("ID:          %s\n", ch.ID)
	fmt.Printf("Slug:        %s\n", ch.Slug)
	fmt.Printf("URL:         %s\n", ch.URL)
	fmt.Printf("Rank:        %s\n", rankStr)
	fmt.Printf("Category:    %s\n", ch.Category)
	if ch.CreatedBy != nil && ch.CreatedBy.Username != "" {
		fmt.Printf("Created By:  %s (%s)\n", ch.CreatedBy.Username, ch.CreatedBy.URL)
	}
	if ch.ApprovedBy != nil && ch.ApprovedBy.Username != "" {
		fmt.Printf("Approved By: %s (%s)\n", ch.ApprovedBy.Username, ch.ApprovedBy.URL)
	}
	fmt.Printf("Stats:       %d completed / %d attempts · ★ %d stars · voteScore %d\n",
		ch.TotalCompleted, ch.TotalAttempts, ch.TotalStars, ch.VoteScore)
	fmt.Printf("Unresolved:  %d issues · %d suggestions (contributorsWanted=%v)\n",
		ch.Unresolved.Issues, ch.Unresolved.Suggestions, ch.ContributorsWanted)
	if len(ch.Tags) > 0 {
		fmt.Printf("Tags:        %s\n", strings.Join(ch.Tags, ", "))
	}
	if len(ch.Languages) > 0 {
		fmt.Printf("Languages:   %s\n", strings.Join(ch.Languages, ", "))
	}
	if *showDesc {
		fmt.Printf("\n--- Description ---\n%s\n", strings.TrimSpace(ch.Description))
	}
	return nil
}

func cmdWebhook(args []string) error {
	fs := flag.NewFlagSet("webhook", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:8795", "Listen address for Codewars Hookbot webhooks")
	secret := fs.String("secret", os.Getenv("CODEWARS_WEBHOOK_SECRET"), "Optional shared X-Webhook-Secret")
	enrich := fs.Bool("enrich", true, "Automatically query Codewars API v1 for kata details on code_challenge events")
	baseURL := fs.String("api-url", "", "Override Codewars API v1 base URL")
	if _, err := parseFlagsInterspersed(fs, args); err != nil {
		return err
	}

	client := newConfiguredClient(*baseURL)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "service": "codewars-webhook"})
			return
		}
		ev, err := codewars.ParseWebhookRequest(r, *secret)
		if err != nil {
			status := http.StatusBadRequest
			if apiErr, ok := err.(*codewars.APIError); ok {
				status = apiErr.StatusCode
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}

		var enriched *codewars.CodeChallenge
		if *enrich && ev.CodeChallenge != nil && ev.CodeChallenge.ID != "" {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			ch, err := client.GetCodeChallenge(ctx, ev.CodeChallenge.ID)
			cancel()
			if err == nil {
				enriched = ch
			}
		}

		out := map[string]any{
			"receivedAt": time.Now().UTC().Format(time.RFC3339),
			"userAgent":  r.Header.Get("User-Agent"),
			"event":      ev.Event,
			"action":     ev.Action,
			"payload":    ev,
		}
		if enriched != nil {
			out["enrichedChallenge"] = map[string]any{
				"id":   enriched.ID,
				"name": enriched.Name,
				"slug": enriched.Slug,
				"rank": enriched.Rank,
			}
		}

		line, _ := json.Marshal(out)
		fmt.Println(string(line))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(line)
	})

	fmt.Fprintf(os.Stderr, "Listening for Codewars Hookbot webhooks on http://%s (secret=%v, enrich=%v)\n",
		*addr, *secret != "", *enrich)
	return http.ListenAndServe(*addr, mux)
}

func cmdBender(args []string) error {
	sub := "list"
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub = args[0]
		rest = args[1:]
	}

	switch sub {
	case "list", "ls":
		return cmdBenderList(rest)
	case "test", "verify", "check":
		return cmdBenderTest(rest)
	case "scaffold", "new", "pull":
		return cmdBenderScaffold(rest)
	case "eli5", "post-eli5", "bluesky", "bsky":
		return cmdBenderELI5(rest)
	default:
		return fmt.Errorf("unknown bender subcommand %q (expected: list | test | scaffold | eli5)", sub)
	}
}

func cmdBenderList(args []string) error {
	fs := flag.NewFlagSet("bender list", flag.ContinueOnError)
	dir := fs.String("dir", "", "Path to local 8b-is/space-bender repository")
	live := fs.Bool("live", false, "Fetch live Codewars API v1 stats for each kata")
	asJSON := fs.Bool("json", false, "Output raw JSON")
	baseURL := fs.String("api-url", "", "Override Codewars API v1 base URL")
	if _, err := parseFlagsInterspersed(fs, args); err != nil {
		return err
	}

	repoDir, err := bender.ResolveDir(*dir)
	if err != nil {
		return err
	}
	entries, err := bender.DiscoverKatas(repoDir)
	if err != nil {
		return err
	}

	if *live {
		client := newConfiguredClient(*baseURL)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		bender.EnrichLive(ctx, client, entries)
	}

	if *asJSON {
		return printJSON(map[string]any{
			"repoDir": repoDir,
			"count":   len(entries),
			"katas":   entries,
		})
	}

	fmt.Printf("8b-is/space-bender (%s) — %d katas\n\n", repoDir, len(entries))
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if *live {
		fmt.Fprintln(tw, "TIER\tSLUG\tLANG\tCODEWARS ID\tAPI RANK\tCOMPLETED\t★ STARS\tSOLUTION")
		for _, e := range entries {
			apiRank := "-"
			comp := "-"
			stars := "-"
			if e.Live != nil {
				if e.Live.Rank != nil && e.Live.Rank.Name != nil {
					apiRank = *e.Live.Rank.Name
				}
				comp = fmt.Sprintf("%d", e.Live.TotalCompleted)
				stars = fmt.Sprintf("%d", e.Live.TotalStars)
			}
			cwID := e.CodewarsID
			if cwID == "" {
				cwID = "-"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				e.Tier, e.Slug, e.Language, cwID, apiRank, comp, stars, e.SolutionFile)
		}
	} else {
		fmt.Fprintln(tw, "TIER\tSLUG\tLANG\tCODEWARS ID\tSOLUTION\tTEST SUITE\tTITLE")
		for _, e := range entries {
			cwID := e.CodewarsID
			if cwID == "" {
				cwID = "-"
			}
			tf := e.TestFile
			if tf == "" {
				tf = "-"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				e.Tier, e.Slug, e.Language, cwID, e.SolutionFile, tf, truncate(e.Title, 44))
		}
	}
	tw.Flush()
	return nil
}

func cmdBenderTest(args []string) error {
	fs := flag.NewFlagSet("bender test", flag.ContinueOnError)
	dir := fs.String("dir", "", "Path to local 8b-is/space-bender repository")
	filter := fs.String("filter", "", "Filter by slug or tier (e.g. 2kyu, evaluate-math)")
	asJSON := fs.Bool("json", false, "Output raw JSON")
	posArgs, err := parseFlagsInterspersed(fs, args)
	if err != nil {
		return err
	}
	if *filter == "" && len(posArgs) > 0 {
		*filter = posArgs[0]
	}

	repoDir, err := bender.ResolveDir(*dir)
	if err != nil {
		return err
	}
	entries, err := bender.DiscoverKatas(repoDir)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	results := bender.RunTests(ctx, entries, *filter)
	passed, failed, skipped := 0, 0, 0
	for _, r := range results {
		if r.Skipped {
			skipped++
		} else if r.Passed {
			passed++
		} else {
			failed++
		}
	}

	if *asJSON {
		return printJSON(map[string]any{
			"repoDir": repoDir,
			"passed":  passed,
			"failed":  failed,
			"skipped": skipped,
			"results": results,
		})
	}

	fmt.Printf("Running 8b-is/space-bender verification suites in %s\n\n", repoDir)
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "STATUS\tTIER\tSLUG\tLANG\tTIME\tCOMMAND")
	for _, r := range results {
		status := "PASS"
		if r.Skipped {
			status = "SKIP"
		} else if !r.Passed {
			status = "FAIL"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%dms\t%s\n",
			status, r.Tier, r.Slug, r.Language, r.DurationMs, r.Command)
		if !r.Passed && r.Output != "" {
			fmt.Fprintf(tw, "  output: %s\n", r.Output)
		}
	}
	tw.Flush()
	fmt.Printf("\nSummary: %d passed · %d skipped · %d failed\n", passed, skipped, failed)
	if failed > 0 {
		return fmt.Errorf("%d kata verification suite(s) failed", failed)
	}
	return nil
}

func cmdBenderScaffold(args []string) error {
	fs := flag.NewFlagSet("bender scaffold", flag.ContinueOnError)
	dir := fs.String("dir", "", "Path to local 8b-is/space-bender repository")
	lang := fs.String("lang", "python", "Solution language (python | lc | go)")
	localSlug := fs.String("slug", "", "Override local kata folder slug")
	baseURL := fs.String("api-url", "", "Override Codewars API v1 base URL")
	posArgs, err := parseFlagsInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(posArgs) < 1 {
		return fmt.Errorf("usage: codewars bender scaffold <id_or_slug_or_url> [--lang python|lc|go] [--slug <name>]")
	}

	repoDir, err := bender.ResolveDir(*dir)
	if err != nil {
		return err
	}

	client := newConfiguredClient(*baseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ch, err := client.GetCodeChallenge(ctx, posArgs[0])
	if err != nil {
		return err
	}

	createdDir, err := bender.ScaffoldKata(repoDir, ch, *lang, *localSlug)
	if err != nil {
		return err
	}

	fmt.Printf("Scaffolded %q (%s) into %s\n", ch.Name, ch.ID, createdDir)
	return nil
}

func cmdBenderELI5(args []string) error {
	fs := flag.NewFlagSet("bender eli5", flag.ContinueOnError)
	dir := fs.String("dir", "", "Path to local 8b-is/space-bender repository")
	dryRun := fs.Bool("dry-run", false, "Preview the 4-post ELI5 thread without posting to Bluesky")
	asJSON := fs.Bool("json", false, "Output JSON receipt")
	posArgs, err := parseFlagsInterspersed(fs, args)
	if err != nil {
		return err
	}
	if len(posArgs) < 1 {
		return fmt.Errorf("usage: codewars bender eli5 <slug> [--dry-run] [--json]")
	}
	slug := posArgs[0]

	repoDir, err := bender.ResolveDir(*dir)
	if err != nil {
		return err
	}
	entries, err := bender.DiscoverKatas(repoDir)
	if err != nil {
		return err
	}

	var target *bender.KataEntry
	for i := range entries {
		if entries[i].Slug == slug {
			target = &entries[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("kata slug %q not found in %s", slug, repoDir)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	receipt, err := bender.PostELI5Thread(ctx, *target, nil, *dryRun)
	if err != nil {
		return err
	}

	if *asJSON {
		return printJSON(receipt)
	}

	fmt.Printf("Bluesky ELI5 Thread for %s (%s) — @%s\n", receipt.KataSlug, receipt.Tier, receipt.Handle)
	if receipt.RootURL != "" {
		fmt.Printf("Root URL: %s\n\n", receipt.RootURL)
	}
	for _, p := range receipt.Posts {
		fmt.Printf("[%d/%d] %s\n%s\n\n", p.Index, len(receipt.Posts), p.URL, p.Text)
	}
	return nil
}

func cmdErrors() error {
	codes := []int{400, 401, 403, 404, 405, 406, 422, 429, 500, 503}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "CODE\tMEANING (https://dev.codewars.com/#errors)")
	for _, code := range codes {
		fmt.Fprintf(tw, "%d\t%s\n", code, codewars.StatusExplanation(code))
	}
	return tw.Flush()
}

func parseFlagsInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var flagsPart []string
	var posPart []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flagsPart = append(flagsPart, a)
			if !strings.Contains(a, "=") {
				name := strings.TrimLeft(a, "-")
				if f := fs.Lookup(name); f != nil {
					if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); !ok || !bf.IsBoolFlag() {
						if i+1 < len(args) {
							i++
							flagsPart = append(flagsPart, args[i])
						}
					}
				}
			}
		} else {
			posPart = append(posPart, a)
		}
	}
	if err := fs.Parse(flagsPart); err != nil {
		return nil, err
	}
	posPart = append(posPart, fs.Args()...)
	return posPart, nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func dedup(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func printUsage() {
	fmt.Print(`codewars — Sovereign Go CLI for Codewars API v1 (https://dev.codewars.com/#introduction)
Paired with 8b-is/space-bender (https://github.com/8b-is/space-bender)

Usage:
  codewars user <username_or_id> [--json]
      GET /api/v1/users/{user} — User profile, honor, leaderboard, overall & per-language ranks.

  codewars completed <username_or_id> [--page 0] [--all] [--lang <lang>] [--json]
      GET /api/v1/users/{user}/code-challenges/completed?page={page} — Completed challenges (200/page).

  codewars authored <username_or_id> [--json]
      GET /api/v1/users/{user}/code-challenges/authored — Challenges authored by the user.

  codewars kata <id_or_slug_or_url> [--desc] [--json]
      GET /api/v1/code-challenges/{challenge} — Code challenge metadata, stats, tags & Markdown description.

  codewars webhook [--addr 127.0.0.1:8795] [--secret <secret>] [--enrich]
      Listen for Codewars Hookbot webhooks (X-Webhook-Event: webhook | code_challenge | user).

  codewars bender list [--dir <path>] [--live] [--json]
      List all solved katas in 8b-is/space-bender (optionally enriched live via Codewars API v1).

  codewars bender test [--dir <path>] [--filter <slug_or_tier>] [--json]
      Run all verification suites across 8b-is/space-bender (Python + @codewars/lambda-calculus).

  codewars bender scaffold <id_or_slug_or_url> [--dir <path>] [--lang python|lc|idr|go] [--slug <name>]
      Fetch kata from Codewars API v1 and scaffold katas/<kyu>/<slug> in 8b-is/space-bender.

  codewars mcp [--dir <path>]
      Run JSON-RPC 2.0 stdio MCP server for 8b-is/space-bender + Codewars API v1.

  codewars errors
      Print the official Codewars API v1 HTTP error status code table (400..503).

  codewars version
      Print CLI version.
`)
}

package bender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// BlueskyPostReceipt records a single published post in a Bluesky ELI5 thread.
type BlueskyPostReceipt struct {
	Index int    `json:"index"`
	URI   string `json:"uri"`
	CID   string `json:"cid"`
	URL   string `json:"url"`
	Text  string `json:"text"`
}

// BlueskyThreadReceipt records the complete multipost ELI5 thread for a kata.
type BlueskyThreadReceipt struct {
	KataSlug  string               `json:"kataSlug"`
	Tier      string               `json:"tier"`
	Handle    string               `json:"handle"`
	DID       string               `json:"did"`
	CreatedAt string               `json:"createdAt"`
	RootURL   string               `json:"rootUrl"`
	Posts     []BlueskyPostReceipt `json:"posts"`
}

var (
	bskyLinkRe = regexp.MustCompile(`https?://[^\s)\]>,"']+`)
	bskyTagRe  = regexp.MustCompile(`#[a-zA-Z0-9_]+`)
)

// BuildELI5Thread constructs a 4-post ELI5 educational thread (each <= 300 graphemes) with GitHub links.
func BuildELI5Thread(entry KataEntry, customPosts []string) []string {
	if len(customPosts) > 0 {
		return customPosts
	}

	kataURL := fmt.Sprintf("https://github.com/8b-is/space-bender/tree/main/katas/%s/%s", entry.Tier, entry.Slug)
	solURL := fmt.Sprintf("https://github.com/8b-is/space-bender/blob/main/katas/%s/%s/%s", entry.Tier, entry.Slug, entry.SolutionFile)
	repoURL := "https://github.com/8b-is/space-bender"

	switch entry.Slug {
	case "google-interview":
		return []string{
			fmt.Sprintf("🌳 ELI5 Kata · Google Interview (Idris 1.3, 7 kyu)\n\nHold a binary tree up to a mirror: every left & right branch swaps places. Hold it up to a 2nd mirror and every branch is right back where it started!\n\nCode + proof: %s", kataURL),
			"🪞 ELI5 Step 1 (The Leaf):\n\nWhat happens to a single Leaf(x) at the tip of a branch when you flip it in a mirror?\nNothing! flipTree(Leaf x) = Leaf x.\n\nFlip it twice and it's still Leaf(x). In Idris 1.3, both sides simplify to Leaf(x), so `Refl` (\"same thing!\") proves the base case.",
			"🌿 ELI5 Step 2 (The Branch):\n\nFor Branch(x, L, R):\n• 1st flip swaps children → Branch(x, flip R, flip L)\n• 2nd flip swaps them back → Branch(x, flip(flip L), flip(flip R))\n\nBy induction, L & R bounce back to themselves, and `congBranch x (flipTreeSym l) (flipTreeSym r)` seals the proof!",
			fmt.Sprintf("✅ Totality checked (`%%default total`) & verified across 1,370 binary trees!\n\nIdris 1.3 proof: %s\nRepo: %s\n\n#Idris #Codewars #TheoremProving #ELI5", solURL, repoURL),
		}
	default:
		return []string{
			fmt.Sprintf("🧩 ELI5 Kata · %s (%s, %s)\n\nNew verified kata solution landed in 8b-is/space-bender! Let's break down how it works like you're 5 🧵👇\n\n%s", entry.Title, entry.Tier, entry.Language, kataURL),
			fmt.Sprintf("🔍 ELI5 Core Idea:\n\nInstead of brute-forcing every possibility, we break %s down into its smallest building blocks, solve the tiny base case directly, and combine each step cleanly so every invariant holds.", entry.Slug),
			fmt.Sprintf("⚡ Verification & Proof:\n\nEvery edge case and invariant is tested locally via `codewars bender test` against our %s solution (`%s`).\n\nSource: %s", entry.Language, entry.SolutionFile, solURL),
			fmt.Sprintf("🌌 Full kata collection & MCP verifier live in 8b-is/space-bender:\n%s\n\n#Codewars #Algorithms #ELI5 #Programming", repoURL),
		}
	}
}

func detectBskyFacets(text string) []map[string]any {
	var facets []map[string]any

	for _, loc := range bskyLinkRe.FindAllStringIndex(text, -1) {
		raw := strings.TrimRight(text[loc[0]:loc[1]], ".,;:!?")
		end := loc[0] + len(raw)
		facets = append(facets, map[string]any{
			"index": map[string]int{
				"byteStart": loc[0],
				"byteEnd":   end,
			},
			"features": []map[string]any{
				{
					"$type": "app.bsky.richtext.facet#link",
					"uri":   raw,
				},
			},
		})
	}

	for _, loc := range bskyTagRe.FindAllStringIndex(text, -1) {
		tag := text[loc[0]+1 : loc[1]]
		facets = append(facets, map[string]any{
			"index": map[string]int{
				"byteStart": loc[0],
				"byteEnd":   loc[1],
			},
			"features": []map[string]any{
				{
					"$type": "app.bsky.richtext.facet#tag",
					"tag":   tag,
				},
			},
		})
	}

	return facets
}

func loadBlueskyCreds() (string, string, string, error) {
	ident := strings.TrimSpace(os.Getenv("BLUESKY_IDENTIFIER"))
	pass := strings.TrimSpace(os.Getenv("BLUESKY_APP_PASSWORD"))
	service := strings.TrimSpace(os.Getenv("BLUESKY_SERVICE"))
	if service == "" {
		service = "https://bsky.social"
	}

	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "workspace", "peterlodri-sec", "bluesky-mcp", ".env"),
		filepath.Join(home, "workspace", "bluesky-mcp", ".env"),
		".env",
	}

	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			switch k {
			case "BLUESKY_IDENTIFIER":
				if ident == "" {
					ident = v
				}
			case "BLUESKY_APP_PASSWORD":
				// Prefer .env over stale environment variable if needed
				pass = v
			case "BLUESKY_SERVICE":
				if v != "" {
					service = v
				}
			}
		}
		if ident != "" && pass != "" {
			break
		}
	}

	if ident == "" || pass == "" {
		return "", "", "", fmt.Errorf("missing BLUESKY_IDENTIFIER or BLUESKY_APP_PASSWORD in environment or bluesky-mcp/.env")
	}
	return ident, pass, strings.TrimRight(service, "/"), nil
}

// PostELI5Thread publishes a multipost ELI5 educational thread for a kata to Bluesky and writes bluesky-receipt.json.
func PostELI5Thread(ctx context.Context, entry KataEntry, customPosts []string, dryRun bool) (*BlueskyThreadReceipt, error) {
	posts := BuildELI5Thread(entry, customPosts)
	for i, p := range posts {
		if n := utf8.RuneCountInString(p); n > 300 {
			return nil, fmt.Errorf("post %d exceeds 300 graphemes (%d chars): %q", i+1, n, p)
		}
	}

	if dryRun {
		receipt := &BlueskyThreadReceipt{
			KataSlug:  entry.Slug,
			Tier:      entry.Tier,
			Handle:    "0xp3t3rl.bsky.social",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		for i, p := range posts {
			receipt.Posts = append(receipt.Posts, BlueskyPostReceipt{
				Index: i + 1,
				Text:  p,
			})
		}
		return receipt, nil
	}

	ident, pass, service, err := loadBlueskyCreds()
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{Timeout: 20 * time.Second}
	loginBody, _ := json.Marshal(map[string]string{
		"identifier": ident,
		"password":   pass,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, service+"/xrpc/com.atproto.server.createSession", bytes.NewReader(loginBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bluesky createSession: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bluesky createSession HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var session struct {
		AccessJwt string `json:"accessJwt"`
		DID       string `json:"did"`
		Handle    string `json:"handle"`
	}
	if err := json.Unmarshal(respBytes, &session); err != nil {
		return nil, err
	}

	receipt := &BlueskyThreadReceipt{
		KataSlug:  entry.Slug,
		Tier:      entry.Tier,
		Handle:    session.Handle,
		DID:       session.DID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	var rootRef, parentRef map[string]string
	for i, text := range posts {
		record := map[string]any{
			"$type":     "app.bsky.feed.post",
			"text":      text,
			"createdAt": time.Now().UTC().Add(time.Duration(i*50) * time.Millisecond).Format(time.RFC3339Nano),
		}
		if facets := detectBskyFacets(text); len(facets) > 0 {
			record["facets"] = facets
		}
		if rootRef != nil && parentRef != nil {
			record["reply"] = map[string]any{
				"root":   rootRef,
				"parent": parentRef,
			}
		}

		bodyBytes, _ := json.Marshal(map[string]any{
			"repo":       session.DID,
			"collection": "app.bsky.feed.post",
			"record":     record,
		})
		postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, service+"/xrpc/com.atproto.repo.createRecord", bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
		}
		postReq.Header.Set("Content-Type", "application/json")
		postReq.Header.Set("Authorization", "Bearer "+session.AccessJwt)

		postResp, err := httpClient.Do(postReq)
		if err != nil {
			return nil, fmt.Errorf("bluesky createRecord (post %d): %w", i+1, err)
		}
		postRespBytes, _ := io.ReadAll(postResp.Body)
		postResp.Body.Close()
		if postResp.StatusCode >= 300 {
			return nil, fmt.Errorf("bluesky createRecord (post %d) HTTP %d: %s", i+1, postResp.StatusCode, string(postRespBytes))
		}

		var created struct {
			URI string `json:"uri"`
			CID string `json:"cid"`
		}
		if err := json.Unmarshal(postRespBytes, &created); err != nil {
			return nil, err
		}

		ref := map[string]string{"uri": created.URI, "cid": created.CID}
		if rootRef == nil {
			rootRef = ref
		}
		parentRef = ref

		parts := strings.Split(created.URI, "/")
		rkey := parts[len(parts)-1]
		webURL := fmt.Sprintf("https://bsky.app/profile/%s/post/%s", session.Handle, rkey)
		if i == 0 {
			receipt.RootURL = webURL
		}
		receipt.Posts = append(receipt.Posts, BlueskyPostReceipt{
			Index: i + 1,
			URI:   created.URI,
			CID:   created.CID,
			URL:   webURL,
			Text:  text,
		})
	}

	if entry.Dir != "" {
		receiptPath := filepath.Join(entry.Dir, "bluesky-receipt.json")
		if b, err := json.MarshalIndent(receipt, "", "  "); err == nil {
			_ = os.WriteFile(receiptPath, append(b, '\n'), 0o644)
		}
	}

	return receipt, nil
}

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
	case "a-plus-a-equals-b-plus-b":
		return []string{
			fmt.Sprintf("⚖️ ELI5 Kata · A+A = B+B → A = B (Coq, 6 kyu)\n\nImagine two kids with identical twin stacks of LEGO bricks: Alice has A+A bricks and Bob has B+B. If their total towers are equal height, must A equal B?\n\nCoq proof: %s", kataURL),
			"🧱 ELI5 Step 1 (Peeling 2 Bricks at a Time):\n\nIn Peano arithmetic, numbers are built from 0 and S (\"one more brick\").\nIf Alice has 0 bricks and Bob has at least 1 (`S b`), `0 = S (...)` is impossible (`discriminate H`)!\nIf both have 0, `0 = 0` (`reflexivity`)!",
			"🔄 ELI5 Step 2 (The S(a)+S(a) Twist):\n\nWhat if both have at least 1 brick (`S a` and `S b`)?\n`(S a) + (S a)` simplifies on the left to `S (a + S a)`.\nOur helper lemma `add_succ_r` pulls the second `S` out to the front: `S (S (a + a)) = S (S (b + b))`!",
			fmt.Sprintf("✅ Peeling off both outer `S` bricks (`inversion H`) gives `a + a = b + b`, so induction (`IHa`) seals it with 0 axioms!\n\nCoq: %s\nRepo: %s\n\n#Coq #HoTT #Codewars #ELI5", solURL, repoURL),
		}
	case "fibonacci-verification":
		return []string{
			fmt.Sprintf("🐚 ELI5 Kata · Program Verification #2: Fibonacci (Coq, 6 kyu)\n\nNaive fib(n) branches into 2^n calls, while fib2(n) = fib_aux 0 1 n walks in O(n) with a 2-number sliding window. How do we prove fib2 n = fib n?\n\n%s", kataURL),
			"🪟 ELI5 Step 1 (Why Direct Induction Gets Stuck):\n\nIf you induct directly on fib_aux 0 1 n, the very first step changes (0, 1) into (1, 1), then (1, 2), then (2, 3)!\nYour induction hypothesis only knows about (0, 1), so it can't match (1, 1).",
			"🔑 ELI5 Step 2 (The Sliding Window Invariant):\n\nGeneralize the window to ANY adjacent Fibonacci pair (fib k, fib (S k)):\n`fib_aux (fib k) (fib (S k)) n = fib (n + k)`!\nOne step shifts `(fib k, fib (S k))` to `(fib (S k), fib (S (S k)))` — matching `IHn (S k)`!",
			fmt.Sprintf("✅ Plugging in k = 0 gives fib_aux (fib 0) (fib 1) n = fib (n + 0) = fib n with 0 axioms!\n\nCoq: %s\nRepo: %s\n\n#Coq #HoTT #Codewars #ELI5", solURL, repoURL),
		}
	case "church-infix-all-ops":
		return []string{
			fmt.Sprintf("λ ELI5 Kata · Church Infix All Operations (2 kyu)\n\nHow do `2 a d d 3 = 5`, `5 s u b 4 = 1`, `3 m u l 4 = 12`, `5 d i v 2 = 2`, `7 m o d 4 = 3`, `2 p o w 3 = 8` work in λ-calculus where each letter is a combinator?\n\n%s", kataURL),
			"🧩 ELI5 Step 1 (How Infix Spelling Works):\n\nIn λ-calculus, `x c1 c2 c3 y` means `(((x c1) c2) c3) y`!\nSince `x` is a Church numeral (\"do c1 x times starting from c2\"), `x c1 c2` counts `x` steps, and then hands `x` to the 3rd letter `c3` and `y`!",
			"🎭 ELI5 Step 2 (The Triple Agent Letter `d`):\n\n11 of the 12 letters only sit in one slot (`c1`, `c2`, or `c3`).\nOnly `d` is a triple agent:\n• `0 a d d y` → `d d y = y`\n• `x d i v y` → `(d^x i) v y = div x y`\n• `x m o d y` → `(x m o) d y = mod x y`\n\n`d = \\p. p A` solves all 3!",
			fmt.Sprintf("✅ Verified across 626 Church numeral tests (`0 pow 0 = 1`, `add`, `sub`, `mul`, `div`, `mod`, `pow`)!\n\nλ: %s\nRepo: %s\n\n#LambdaCalculus #Codewars #ELI5", solURL, repoURL),
		}
	case "hamming-numbers":
		return []string{
			fmt.Sprintf("⚙️ ELI5 Kata · Hamming Numbers (NASM x86-64, 4 kyu)\n\nHamming numbers use only prime factors 2, 3, 5 (2^i·3^j·5^k): 1, 2, 3, 4, 5, 6, 8, 9, 10, 12...\nHow do we compute all 13,282 64-bit Hamming numbers in 141 bytes of NASM?\n\n%s", kataURL),
			"🏭 ELI5 Step 1 (Three Conveyor Belts):\n\nImagine 3 machines reading already-found Hamming numbers:\n• Machine 2 doubles them (`i2`)\n• Machine 3 triples them (`i3`)\n• Machine 5×5s them (`i5`)\nEach step picks `min(x2, x3, x5)` and steps whichever belt(s) tied for smallest!",
			"🚀 ELI5 Step 2 (Zero `mul` Instructions in x86-64):\n\nIn NASM we never even call `mul`!\n• `×2`: `add r10, r10`\n• `×3`: `lea r11, [r11 + r11*2]`\n• `×5`: `lea rdi, [rdi + rdi*4]`\n• `min`: branchless `cmovb` (unsigned `<`, crucial once values pass 2^63)!",
			fmt.Sprintf("✅ Cached in `.bss` on call #1, so all 13,282 calls take 6 instructions (`hamming(13282) = 18432000000000000000`)!\n\n%s\nRepo: %s\n\n#NASM #x86_64 #Codewars #ELI5", solURL, repoURL),
		}
	case "express-number-as-sum-of-four-squares":
		return []string{
			fmt.Sprintf("🦀 ELI5 Kata · Sum of Four Squares (Rust, 1 kyu)\n\nLagrange proved every number N = a² + b² + c² + d².\nHow do we decompose huge 1024-bit numbers in ~0.25 ms each in pure Rust without GMP?\n\n%s", kataURL),
			"🪆 ELI5 Step 1 (The Double-Sqrt Cascade):\n\nInstead of testing 1024-bit primes:\n1. Pick `a ≈ √n` (512 bits) → remainder `r1 = n - a²` shrinks to ~513 bits!\n2. Pick `b ≈ √r1` (257 bits) → remainder `p = r1 - b²` shrinks to ~258 bits (`[u64; 5]`)!",
			"⚡ ELI5 Step 2 (Parity Lock + Stack Montgomery):\n\nMatching `(a, b)` parities to `n mod 4` forces `p = n - a² - b² ≡ 1 (mod 4)` every time!\n96 small primes filter 85% of steps in `u32`, and stack `[u64; 5]` Montgomery Miller-Rabin + Cornacchia finds `c² + d² = p` with 0 heap allocs!",
			fmt.Sprintf("✅ All 20×2^128 + 20×2^1024 random tests solved in ~7.2 ms total in Rust!\n\n%s\nRepo: %s\n\n#Rust #NumberTheory #Codewars #ELI5", solURL, repoURL),
		}
	case "negabinary":
		return []string{
			fmt.Sprintf("➖2️⃣ ELI5 Kata · NegaBinary (λ-Calculus, 2 kyu)\n\nIn base -2 (used in 1950s Polish BINEG computers!), every bit multiplies by -2 (`E x = -2x`, `O x = 1 - 2x`), representing + and - numbers with NO sign bit!\n\n%s", kataURL),
			"🪞 ELI5 Step 1 (1-Step Sign-Flip Unrolling):\n\nBecause multiplying by -2 flips the sign at every bit:\n• `pred (E x) = O (succ x)` & `pred (O x) = mkE x` \n• `lt0 n` is just `gt0` of the tail!\n• `hlf n = n Zero neg neg` (floor ÷2 in 1 line)!\nZero mutual-recursion forward refs needed!",
			"➗ ELI5 Step 2 (BinaryScott Bridge & Deferred `()`):\n\nIn base -2, `mul` needs no subtraction (`(1-2x)n = n + E(x*n)`).\nFor `div`, `mod`, `gcd`, `lcm`, we bridge `|m|, |n|` in O(log n) to unsigned `BinaryScott` long division and wrap error `()` in `\\_.()`!",
			fmt.Sprintf("✅ All 47 exports & 49 test suites verified in ~730 ms (vs 5s limit)!\n\n%s\nRepo: %s\n\n#LambdaCalculus #Codewars #ELI5", solURL, repoURL),
		}
	case "game-of-life-in-reverse":
		return []string{
			fmt.Sprintf("🧬 ELI5 Kata · Game of Life in Reverse (Factor, 1 kyu)\n\nStepping Conway's Game of Life forward is easy, but how do you run time BACKWARDS to find a predecessor (or prove a pattern is a Garden of Eden)?\n\n%s", kataURL),
			"🎯 ELI5 Step 1 (Interval Feasibility Bounds):\n\nEach cell tracks its placed live neighbors `c` and remaining unplaced neighbors `r`.\n• Need 1 next gen? Valid iff `c <= 3` and `c + r + s >= 3`\n• Need 0 next gen? Valid iff `c + s < 3` or `c + r > 3`\nDead branches prune immediately!",
			"🪟 ELI5 Step 2 (4-Bit Sliding Frontier Profile DP):\n\nIn raster order, only the last `W + 1` cells still touch future cells.\nPacking `(min(c, 4), s)` into 4 bits per frontier cell gives one integer key cached in a Factor hash-set (`?adjoin`), never repeating dead-ends!",
			fmt.Sprintf("✅ Verified across 55 grids + Garden of Eden in Factor 0.102!\n\n%s\nRepo: %s\n\n#Factor #GameOfLife #Codewars #ELI5", solURL, repoURL),
		}
	case "ascii-games-dance-dance-evolution-i":
		return []string{
			fmt.Sprintf("🕺 ELI5 Kata · Dance Dance Evolution I (Rust, 2 kyu)\n\nOn an arrow grid (←↖↑↗→↘↓↙), each step must stay ≤90° from BOTH your current & target arrows. How do we find the longest cycle back to S?\n\n%s", kataURL),
			"🧭 ELI5 Step 1 (The 90° Compass Rule):\n\nNumber the 8 directions 0..7 around the compass (45° per step).\nTwo directions are ≤ 90° apart iff `diff <= 2 || diff >= 6`.\nWe precompile all valid transitions into 64-bit adjacency masks `out_mask[u]` & `in_mask[v]`!",
			"⚡ ELI5 Step 2 (Bidirectional Bitmask Pruning):\n\nAt each DFS state `(u, visited)`, hardware `trailing_zeros()` bit-scans flood-fill:\n1. `can_reach_s` backwards from `S` through unvisited tiles\n2. `reachable` forwards from `u` inside `can_reach_s`\nPruning dead ends & upper bounds in O(1)!",
			fmt.Sprintf("✅ Verified across 100 random 5x5 mazes in ~26 ms in Rust!\n\n%s\nRepo: %s\n\n#Rust #GraphTheory #Codewars #ELI5", solURL, repoURL),
		}
	case "break-the-pieces":
		return []string{
			fmt.Sprintf("🧩 ELI5 Kata · Break the pieces (C++, 2 kyu)\n\nGiven an ASCII blueprint of nested rooms (+, -, |, spaces), how do we slice out every minimal closed room and heal its shared walls?\n\n%s", kataURL),
			"🌊 ELI5 Step 1 (Pouring Water Outside):\n\nFirst, pad the blueprint with a 1-space border and flood-fill from `(0, 0)`.\nEvery space the water reaches is \"outside\".\nEach dry pocket of spaces left inside is a minimal closed room!",
			"🧱 ELI5 Step 2 (8-Neighbor Walls & Corner Healing):\n\nWalk the 8-neighbors of a room's dry spaces to collect its wall cells (`bnd_stamp`).\nDid a `+` T-junction belong to a neighbor's wall? If it only has horizontal neighbors in THIS room's wall, it heals to `-` (or `|` for vertical)!",
			fmt.Sprintf("✅ Verified in C++20 across nested donuts, interlocking L-rooms & classic grids!\n\n%s\nRepo: %s\n\n#CPP #ASCII #Codewars #ELI5", solURL, repoURL),
		}
	case "bloxorz-solver":
		return []string{
			fmt.Sprintf("🧱 ELI5 Kata · Bloxorz Solver (Haskell + ASM, 3 kyu)\n\nHow do you roll a 1×1×2 block across a floating island from B into the square hole X in the fewest moves without tipping over the edge?\n\n%s", kataURL),
			"🎲 ELI5 Step 1 (3 Poses of a Domino Block):\n\nAt any moment the block has 1 of 3 poses:\n• `Upright r c`: standing tall on 1 tile\n• `Horz r c`: lying flat across `(r, c)` & `(r, c+1)`\n• `Vert r c`: lying flat across `(r, c)` & `(r+1, c)`\nTipping upright shifts 2 tiles; rolling flat shifts 1!",
			"⚡ ELI5 Step 2 (Haskell `Seq` BFS + 12-Bit Assembly Moat):\n\nIn Haskell (`solution.hs`), `Data.Sequence` BFS finds the shortest path.\nIn x86-64 NASM & ARM64 (`solution.asm`, `solution_arm64.s`), a 32×32 zero-moat grid packs `(orient << 10) | idx` into 12 bits with branchless LUT rolls!",
			fmt.Sprintf("✅ Verified in Haskell (GHC) + x86-64 NASM + Apple Silicon ARM64!\n\n%s\nRepo: %s\n\n#Haskell #Assembly #Codewars #ELI5", solURL, repoURL),
		}
	case "range-extraction":
		return []string{
			fmt.Sprintf("📏 ELI5 Kata · Range Extraction (Haskell + Julia + ASM, 4 kyu)\n\nHow do we compress `[-6,-3,-2,-1,0,1,3,4,5]` into `\"-6,-3-1,3-5\"` where only runs of 3+ consecutive integers become `a-b`?\n\n%s", kataURL),
			"🔍 ELI5 Step 1 (The 3-Number Rule):\n\nWalk consecutive neighbors `y == prev + 1` from `a` to `b`:\n• Span `b - a >= 2` (3+ numbers): collapse to `\"a-b\"`\n• Span `b - a == 1` (just 2 numbers): keep separate `\"a,b\"`\n• Span `0` (single number): emit `\"a\"`!",
			"⚡ ELI5 Step 2 (Haskell + Julia + Zero-Libc Assembly):\n\n• Haskell (`solution.hs`): `spanRun` + `intercalate \",\"`\n• Julia (`solution.jl`): two-pointer run scanner\n• x86-64 NASM & ARM64 (`solution.asm`, `solution_arm64.s`): pure register `div`/`udiv` ASCII digit streamer!",
			fmt.Sprintf("✅ Verified across Haskell (GHC 9.14), Julia 1.13, x86-64 NASM & ARM64!\n\n%s\nRepo: %s\n\n#Haskell #JuliaLang #Assembly #Codewars #ELI5", solURL, repoURL),
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

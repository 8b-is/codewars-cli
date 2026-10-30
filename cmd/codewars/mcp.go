package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/8b-is/codewars-cli/pkg/bender"
)

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func cmdMCP(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	dir := fs.String("dir", "", "Path to local 8b-is/space-bender repository")
	baseURL := fs.String("api-url", "", "Override Codewars API v1 base URL")
	if _, err := parseFlagsInterspersed(fs, args); err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	enc := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		if len(req.ID) == 0 {
			// JSON-RPC notification (e.g. notifications/initialized)
			continue
		}
		resp := handleMCPRequest(req, *dir, *baseURL)
		if err := enc.Encode(resp); err != nil && err != io.EOF {
			return err
		}
	}
	return scanner.Err()
}

func handleMCPRequest(req jsonRPCRequest, defaultDir, baseURL string) jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
				"serverInfo": map[string]any{
					"name":    "space-bender-mcp",
					"version": version,
				},
			},
		}
	case "ping":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}
	case "tools/list":
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": mcpToolsSchema(),
			},
		}
	case "tools/call":
		var callParams struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &callParams); err != nil {
			return jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &jsonRPCError{Code: -32602, Message: err.Error()},
			}
		}
		payload, err := dispatchMCPTool(callParams.Name, callParams.Arguments, defaultDir, baseURL)
		isErr := err != nil
		if err != nil {
			payload = map[string]any{"error": err.Error()}
		}
		textBytes, _ := json.MarshalIndent(payload, "", "  ")
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{
						"type": "text",
						"text": string(textBytes),
					},
				},
				"isError": isErr,
			},
		}
	default:
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32601, Message: "Method not found: " + req.Method},
		}
	}
}

func mcpToolsSchema() []map[string]any {
	return []map[string]any{
		{
			"name":        "bender_list",
			"description": "List all solved katas in 8b-is/space-bender (optionally enriched live via Codewars API v1).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"live": map[string]any{
						"type":        "boolean",
						"description": "Fetch live Codewars API v1 stats for each kata.",
					},
				},
			},
		},
		{
			"name":        "bender_test",
			"description": "Run local verification suites across 8b-is/space-bender katas.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filter": map[string]any{
						"type":        "string",
						"description": "Optional slug or tier filter (e.g. 'google-interview', '7kyu', '2kyu').",
					},
				},
			},
		},
		{
			"name":        "bender_read_solution",
			"description": "Read the solution source file and README.md for a kata in 8b-is/space-bender.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"slug": map[string]any{
						"type":        "string",
						"description": "Kata directory slug (e.g. 'google-interview', 'evaluate-math').",
					},
				},
				"required": []string{"slug"},
			},
		},
		{
			"name":        "bender_post_eli5",
			"description": "Publish a multipost ELI5 educational Bluesky thread (with GitHub links) for a solved kata in 8b-is/space-bender and save bluesky-receipt.json.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"slug": map[string]any{
						"type":        "string",
						"description": "Kata directory slug (e.g. 'google-interview').",
					},
					"dry_run": map[string]any{
						"type":        "boolean",
						"description": "If true, preview the ELI5 thread without posting live to Bluesky.",
					},
				},
				"required": []string{"slug"},
			},
		},
		{
			"name":        "codewars_kata",
			"description": "Query Codewars API v1 GET /api/v1/code-challenges/{challenge} by ID, slug, or URL.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"challenge": map[string]any{
						"type":        "string",
						"description": "Codewars kata ID, slug, or full URL.",
					},
				},
				"required": []string{"challenge"},
			},
		},
		{
			"name":        "codewars_user",
			"description": "Query Codewars API v1 GET /api/v1/users/{user} profile and ranks.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"user": map[string]any{
						"type":        "string",
						"description": "Codewars username or ID.",
					},
				},
				"required": []string{"user"},
			},
		},
	}
}

func dispatchMCPTool(name string, args map[string]any, defaultDir, baseURL string) (any, error) {
	switch name {
	case "bender_list":
		repoDir, err := bender.ResolveDir(defaultDir)
		if err != nil {
			return nil, err
		}
		entries, err := bender.DiscoverKatas(repoDir)
		if err != nil {
			return nil, err
		}
		if live, ok := args["live"].(bool); ok && live {
			client := newConfiguredClient(baseURL)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			bender.EnrichLive(ctx, client, entries)
		}
		return map[string]any{
			"repoDir": repoDir,
			"count":   len(entries),
			"katas":   entries,
		}, nil

	case "bender_test":
		repoDir, err := bender.ResolveDir(defaultDir)
		if err != nil {
			return nil, err
		}
		entries, err := bender.DiscoverKatas(repoDir)
		if err != nil {
			return nil, err
		}
		filter, _ := args["filter"].(string)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		results := bender.RunTests(ctx, entries, filter)
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
		return map[string]any{
			"repoDir": repoDir,
			"passed":  passed,
			"failed":  failed,
			"skipped": skipped,
			"results": results,
		}, nil

	case "bender_read_solution":
		slug, _ := args["slug"].(string)
		if slug == "" {
			return nil, fmt.Errorf("missing required parameter 'slug'")
		}
		repoDir, err := bender.ResolveDir(defaultDir)
		if err != nil {
			return nil, err
		}
		entries, err := bender.DiscoverKatas(repoDir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Slug == slug {
				solBytes, _ := os.ReadFile(filepath.Join(e.Dir, e.SolutionFile))
				readmeBytes, _ := os.ReadFile(filepath.Join(e.Dir, "README.md"))
				return map[string]any{
					"kata":     e,
					"solution": string(solBytes),
					"readme":   string(readmeBytes),
				}, nil
			}
		}
		return nil, fmt.Errorf("kata slug %q not found in %s", slug, repoDir)

	case "bender_post_eli5":
		slug, _ := args["slug"].(string)
		if slug == "" {
			return nil, fmt.Errorf("missing required parameter 'slug'")
		}
		dryRun, _ := args["dry_run"].(bool)
		repoDir, err := bender.ResolveDir(defaultDir)
		if err != nil {
			return nil, err
		}
		entries, err := bender.DiscoverKatas(repoDir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.Slug == slug {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				return bender.PostELI5Thread(ctx, e, nil, dryRun)
			}
		}
		return nil, fmt.Errorf("kata slug %q not found in %s", slug, repoDir)

	case "codewars_kata":
		query, _ := args["challenge"].(string)
		if query == "" {
			return nil, fmt.Errorf("missing required parameter 'challenge'")
		}
		if mapped, ok := bender.KnownKataIDs[query]; ok {
			query = mapped
		}
		client := newConfiguredClient(baseURL)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return client.GetCodeChallenge(ctx, query)

	case "codewars_user":
		user, _ := args["user"].(string)
		if user == "" {
			return nil, fmt.Errorf("missing required parameter 'user'")
		}
		client := newConfiguredClient(baseURL)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return client.GetUser(ctx, user)

	default:
		return nil, fmt.Errorf("unknown tool %q", name)
	}
}

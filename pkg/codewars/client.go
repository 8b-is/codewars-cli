package codewars

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the official Codewars API v1 base URL.
	DefaultBaseURL = "https://www.codewars.com/api/v1"
	// DefaultUserAgent identifies this CLI to the Codewars API.
	DefaultUserAgent = "codewars-cli/1.0 (+https://github.com/8b-is/codewars-cli; space-bender)"
)

// Client is a pure-stdlib Go client for the Codewars API v1 (https://dev.codewars.com/#introduction).
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
	APIKey     string
}

// NewClient returns a Client configured for the official Codewars API v1.
func NewClient() *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		UserAgent: DefaultUserAgent,
	}
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = DefaultBaseURL
	}
	fullURL := base + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("create request %s: %w", fullURL, err)
	}
	req.Header.Set("Accept", "application/json")
	ua := c.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	if c.APIKey != "" {
		req.Header.Set("Authorization", c.APIKey)
	}

	hc := c.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}

	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", fullURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("read response %s: %w", fullURL, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errPayload struct {
			Reason  string `json:"reason"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &errPayload)
		reason := errPayload.Reason
		if reason == "" {
			reason = errPayload.Message
		}
		return &APIError{
			StatusCode:  resp.StatusCode,
			Reason:      reason,
			Explanation: StatusExplanation(resp.StatusCode),
			URL:         fullURL,
		}
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode json from %s: %w", fullURL, err)
	}
	return nil
}

// GetUser fetches user information from GET /api/v1/users/{user}.
// user may be a username or user ID.
func (c *Client) GetUser(ctx context.Context, user string) (*User, error) {
	user = strings.TrimSpace(user)
	if user == "" {
		return nil, fmt.Errorf("user is required")
	}
	var out User
	if err := c.getJSON(ctx, "/users/"+url.PathEscape(user), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListCompletedChallenges fetches one page (up to 200 items, zero-based page offset)
// of completed challenges from GET /api/v1/users/{user}/code-challenges/completed?page={page}.
func (c *Client) ListCompletedChallenges(ctx context.Context, user string, page int) (*CompletedChallengesPage, error) {
	user = strings.TrimSpace(user)
	if user == "" {
		return nil, fmt.Errorf("user is required")
	}
	if page < 0 {
		page = 0
	}
	path := fmt.Sprintf("/users/%s/code-challenges/completed?page=%d", url.PathEscape(user), page)
	var out CompletedChallengesPage
	if err := c.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAllCompletedChallenges iterates through all pages (up to maxPages if > 0)
// of a user's completed challenges.
func (c *Client) ListAllCompletedChallenges(ctx context.Context, user string, maxPages int) ([]CompletedChallenge, int, error) {
	first, err := c.ListCompletedChallenges(ctx, user, 0)
	if err != nil {
		return nil, 0, err
	}
	all := make([]CompletedChallenge, 0, first.TotalItems)
	all = append(all, first.Data...)
	totalPages := first.TotalPages
	if maxPages > 0 && totalPages > maxPages {
		totalPages = maxPages
	}
	for page := 1; page < totalPages; page++ {
		next, err := c.ListCompletedChallenges(ctx, user, page)
		if err != nil {
			return all, first.TotalItems, err
		}
		all = append(all, next.Data...)
	}
	return all, first.TotalItems, nil
}

// ListAuthoredChallenges fetches challenges authored by a user from
// GET /api/v1/users/{user}/code-challenges/authored.
func (c *Client) ListAuthoredChallenges(ctx context.Context, user string) ([]AuthoredChallenge, error) {
	user = strings.TrimSpace(user)
	if user == "" {
		return nil, fmt.Errorf("user is required")
	}
	path := fmt.Sprintf("/users/%s/code-challenges/authored", url.PathEscape(user))
	var out AuthoredChallengesResponse
	if err := c.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// GetCodeChallenge fetches a code challenge (kata) by ID or slug from
// GET /api/v1/code-challenges/{challenge}.
func (c *Client) GetCodeChallenge(ctx context.Context, challenge string) (*CodeChallenge, error) {
	challenge = strings.TrimSpace(challenge)
	if challenge == "" {
		return nil, fmt.Errorf("challenge id or slug is required")
	}
	// Accept full Codewars kata URLs as well as raw IDs or slugs.
	if strings.Contains(challenge, "codewars.com/kata/") {
		parts := strings.Split(challenge, "codewars.com/kata/")
		rest := strings.Trim(parts[len(parts)-1], "/")
		if idx := strings.IndexByte(rest, '/'); idx >= 0 {
			rest = rest[:idx]
		}
		challenge = rest
	}
	path := "/code-challenges/" + url.PathEscape(challenge)
	var out CodeChallenge
	if err := c.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

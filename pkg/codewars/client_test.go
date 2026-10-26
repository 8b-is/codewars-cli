package codewars

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientEndpoints(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/users/some_user":
			w.Write([]byte(`{
				"username": "some_user",
				"name": "Some Person",
				"honor": 544,
				"clan": "some clan",
				"leaderboardPosition": 134,
				"skills": ["go", "python"],
				"ranks": {
					"overall": {"rank": -3, "name": "3 kyu", "color": "blue", "score": 2116},
					"languages": {
						"go": {"rank": -3, "name": "3 kyu", "color": "blue", "score": 1819}
					}
				},
				"codeChallenges": {"totalAuthored": 3, "totalCompleted": 230}
			}`))
		case r.URL.Path == "/users/some_user/code-challenges/completed":
			page := r.URL.Query().Get("page")
			if page == "1" {
				w.Write([]byte(`{
					"totalPages": 2,
					"totalItems": 2,
					"data": [{"id": "52a78825cdfc2cfc87000005", "name": "Evaluate mathematical expression", "slug": "evaluate-mathematical-expression", "completedAt": "2026-10-16T12:00:00Z", "completedLanguages": ["python"]}]
				}`))
				return
			}
			w.Write([]byte(`{
				"totalPages": 2,
				"totalItems": 2,
				"data": [{"id": "514b92a657cdc65150000006", "name": "Multiples of 3 and 5", "slug": "multiples-of-3-and-5", "completedAt": "2017-04-06T16:32:09Z", "completedLanguages": ["javascript", "go"]}]
			}`))
		case r.URL.Path == "/users/some_user/code-challenges/authored":
			w.Write([]byte(`{
				"data": [
					{"id": "5571d9fc11526780a000011a", "name": "The builder of things", "description": "Meta...", "rank": -3, "rankName": "3 kyu", "tags": ["Algorithms"], "languages": ["ruby", "go"]}
				]
			}`))
		case r.URL.Path == "/code-challenges/valid-braces" || r.URL.Path == "/code-challenges/5277c8a221e209d3f6000b56":
			w.Write([]byte(`{
				"id": "5277c8a221e209d3f6000b56",
				"name": "Valid Braces",
				"slug": "valid-braces",
				"url": "http://www.codewars.com/kata/valid-braces",
				"category": "algorithms",
				"description": "Write a function...",
				"tags": ["Algorithms", "Validation"],
				"languages": ["javascript", "go"],
				"rank": {"id": -4, "name": "4 kyu", "color": "blue"},
				"createdBy": {"username": "xDranik", "url": "http://www.codewars.com/users/xDranik"},
				"totalAttempts": 4911,
				"totalCompleted": 919,
				"totalStars": 12,
				"voteScore": 512,
				"contributorsWanted": true,
				"unresolved": {"issues": 1, "suggestions": 2}
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"reason":"not found"}`))
		}
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL
	ctx := context.Background()

	u, err := c.GetUser(ctx, "some_user")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u.Username != "some_user" || u.Honor != 544 || u.Ranks.Overall.Name != "3 kyu" {
		t.Fatalf("unexpected user: %+v", u)
	}

	allComp, total, err := c.ListAllCompletedChallenges(ctx, "some_user", 5)
	if err != nil {
		t.Fatalf("ListAllCompletedChallenges: %v", err)
	}
	if total != 2 || len(allComp) != 2 {
		t.Fatalf("expected 2 completed items across 2 pages, got %d (total=%d)", len(allComp), total)
	}

	auth, err := c.ListAuthoredChallenges(ctx, "some_user")
	if err != nil {
		t.Fatalf("ListAuthoredChallenges: %v", err)
	}
	if len(auth) != 1 || auth[0].Name != "The builder of things" {
		t.Fatalf("unexpected authored: %+v", auth)
	}

	ch, err := c.GetCodeChallenge(ctx, "https://www.codewars.com/kata/5277c8a221e209d3f6000b56/train/python")
	if err != nil {
		t.Fatalf("GetCodeChallenge URL: %v", err)
	}
	if ch.Slug != "valid-braces" || ch.TotalCompleted != 919 || ch.Unresolved.Suggestions != 2 {
		t.Fatalf("unexpected challenge: %+v", ch)
	}

	_, err = c.GetUser(ctx, "missing_user")
	if err == nil {
		t.Fatal("expected 404 error")
	}
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != 404 || !strings.Contains(apiErr.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebhooksAndErrors(t *testing.T) {
	codes := []int{400, 401, 403, 404, 405, 406, 422, 429, 500, 503}
	for _, code := range codes {
		exp := StatusExplanation(code)
		if exp == "" || strings.HasPrefix(exp, "HTTP ") {
			t.Fatalf("missing explanation for %d: %s", code, exp)
		}
	}

	payload := `{"action":"solution_finalized","code_challenge":{"id":"53aa3f265b97485984000001","created_by_id":"53af25145b97487568000001"},"solution":{"id":"53aa3f265b97485984000001","user_id":"53417de006654f4171000587"}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(payload))
	req.Header.Set("User-Agent", "Codewars Hookbot")
	req.Header.Set("X-Webhook-Event", "code_challenge")
	req.Header.Set("X-Webhook-Secret", "s3cr3t")

	ev, err := ParseWebhookRequest(req, "s3cr3t")
	if err != nil {
		t.Fatalf("ParseWebhookRequest: %v", err)
	}
	if ev.Event != "code_challenge" || ev.Action != "solution_finalized" || ev.Solution == nil || ev.Solution.UserID != "53417de006654f4171000587" {
		t.Fatalf("unexpected webhook event: %+v", ev)
	}

	reqBad := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(payload))
	reqBad.Header.Set("X-Webhook-Secret", "wrong")
	if _, err := ParseWebhookRequest(reqBad, "s3cr3t"); err == nil {
		t.Fatal("expected secret mismatch error")
	}
}

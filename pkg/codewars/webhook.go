package codewars

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// WebhookEvent represents a parsed Codewars Hookbot webhook payload
// (https://dev.codewars.com/#webhooks).
type WebhookEvent struct {
	Event         string                 `json:"event"`
	Action        string                 `json:"action"`
	Webhook       *WebhookRef            `json:"webhook,omitempty"`
	CodeChallenge *WebhookChallengeRef   `json:"code_challenge,omitempty"`
	Solution      *WebhookSolutionRef    `json:"solution,omitempty"`
	User          *WebhookUserPayload    `json:"user,omitempty"`
	Language      *string                `json:"language,omitempty"`
	Raw           map[string]any         `json:"raw,omitempty"`
}

// WebhookRef identifies the webhook registration in a ping/updated event.
type WebhookRef struct {
	ID string `json:"id"`
}

// WebhookChallengeRef identifies a kata in a code_challenge webhook event.
type WebhookChallengeRef struct {
	ID          string `json:"id"`
	CreatedByID string `json:"created_by_id,omitempty"`
}

// WebhookSolutionRef identifies a solution in a solution_finalized webhook event.
type WebhookSolutionRef struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
}

// WebhookUserPayload carries user rank or honor updates in a user webhook event.
type WebhookUserPayload struct {
	ID         string `json:"id"`
	Rank       *int   `json:"rank,omitempty"`
	Honor      *int   `json:"honor,omitempty"`
	HonorDelta *int   `json:"honor_delta,omitempty"`
}

// ParseWebhookRequest verifies (if secret is non-empty) and parses an inbound
// Codewars webhook HTTP request.
func ParseWebhookRequest(r *http.Request, expectedSecret string) (*WebhookEvent, error) {
	if r.Method != http.MethodPost {
		return nil, &APIError{
			StatusCode:  http.StatusMethodNotAllowed,
			Explanation: StatusExplanation(http.StatusMethodNotAllowed),
			Reason:      "webhooks require POST",
			URL:         r.URL.String(),
		}
	}

	if expectedSecret != "" {
		gotSecret := r.Header.Get("X-Webhook-Secret")
		if subtle.ConstantTimeCompare([]byte(gotSecret), []byte(expectedSecret)) != 1 {
			return nil, &APIError{
				StatusCode:  http.StatusUnauthorized,
				Explanation: StatusExplanation(http.StatusUnauthorized),
				Reason:      "invalid X-Webhook-Secret header",
				URL:         r.URL.String(),
			}
		}
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read webhook body: %w", err)
	}

	var ev WebhookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return nil, &APIError{
			StatusCode:  http.StatusBadRequest,
			Explanation: StatusExplanation(http.StatusBadRequest),
			Reason:      fmt.Sprintf("invalid json body: %v", err),
			URL:         r.URL.String(),
		}
	}

	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err == nil {
		ev.Raw = raw
	}

	eventHeader := r.Header.Get("X-Webhook-Event")
	if eventHeader != "" {
		ev.Event = eventHeader
	} else if ev.Event == "" {
		switch {
		case ev.Webhook != nil:
			ev.Event = "webhook"
		case ev.CodeChallenge != nil:
			ev.Event = "code_challenge"
		case ev.Solution != nil:
			ev.Event = "solution"
		case ev.User != nil:
			ev.Event = "user"
		default:
			ev.Event = "unknown"
		}
	}

	return &ev, nil
}

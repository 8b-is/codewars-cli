package codewars

import "fmt"

// User represents a Codewars user profile returned by GET /api/v1/users/{user}.
type User struct {
	Username            string              `json:"username"`
	Name                string              `json:"name"`
	Honor               int                 `json:"honor"`
	Clan                string              `json:"clan"`
	LeaderboardPosition int                 `json:"leaderboardPosition"`
	Skills              []string            `json:"skills"`
	Ranks               Ranks               `json:"ranks"`
	CodeChallenges      CodeChallengesCount `json:"codeChallenges"`
}

// Ranks holds a user's overall rank and per-language ranks.
type Ranks struct {
	Overall   RankInfo            `json:"overall"`
	Languages map[string]RankInfo `json:"languages"`
}

// RankInfo describes a rank in kyu [-8..-1] or dan [1..8].
type RankInfo struct {
	Rank  int    `json:"rank"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Score int    `json:"score"`
}

// CodeChallengesCount holds authored and completed kata counts for a user.
type CodeChallengesCount struct {
	TotalAuthored  int `json:"totalAuthored"`
	TotalCompleted int `json:"totalCompleted"`
}

// CompletedChallengesPage represents a paginated response from
// GET /api/v1/users/{user}/code-challenges/completed?page={page}.
type CompletedChallengesPage struct {
	TotalPages int                  `json:"totalPages"`
	TotalItems int                  `json:"totalItems"`
	Data       []CompletedChallenge `json:"data"`
}

// CompletedChallenge represents a single completed kata entry.
type CompletedChallenge struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Slug               string   `json:"slug"`
	CompletedAt        string   `json:"completedAt"`
	CompletedLanguages []string `json:"completedLanguages"`
}

// AuthoredChallengesResponse represents the response from
// GET /api/v1/users/{user}/code-challenges/authored.
type AuthoredChallengesResponse struct {
	Data []AuthoredChallenge `json:"data"`
}

// AuthoredChallenge represents a single kata authored by a user.
type AuthoredChallenge struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Rank        *int     `json:"rank,omitempty"`
	RankName    *string  `json:"rankName,omitempty"`
	Tags        []string `json:"tags"`
	Languages   []string `json:"languages"`
}

// CodeChallenge represents full kata metadata from
// GET /api/v1/code-challenges/{challenge}.
type CodeChallenge struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Slug               string          `json:"slug"`
	URL                string          `json:"url"`
	Category           string          `json:"category"`
	Description        string          `json:"description"`
	Tags               []string        `json:"tags"`
	Languages          []string        `json:"languages"`
	Rank               *ChallengeRank  `json:"rank,omitempty"`
	CreatedBy          *UserRef        `json:"createdBy,omitempty"`
	ApprovedBy         *UserRef        `json:"approvedBy,omitempty"`
	PublishedAt        string          `json:"publishedAt,omitempty"`
	ApprovedAt         string          `json:"approvedAt,omitempty"`
	TotalAttempts      int             `json:"totalAttempts"`
	TotalCompleted     int             `json:"totalCompleted"`
	TotalStars         int             `json:"totalStars"`
	VoteScore          int             `json:"voteScore"`
	ContributorsWanted bool            `json:"contributorsWanted"`
	Unresolved         UnresolvedCount `json:"unresolved"`
}

// ChallengeRank describes a kata's rank object (note: uses "id" instead of "rank").
type ChallengeRank struct {
	ID    *int    `json:"id,omitempty"`
	Name  *string `json:"name,omitempty"`
	Color *string `json:"color,omitempty"`
}

// UserRef references an author or approver on a CodeChallenge.
type UserRef struct {
	Username string `json:"username"`
	URL      string `json:"url"`
}

// UnresolvedCount holds unresolved issues and suggestions counts on a kata.
type UnresolvedCount struct {
	Issues      int `json:"issues"`
	Suggestions int `json:"suggestions"`
}

// APIError represents an error returned by the Codewars API v1.
type APIError struct {
	StatusCode  int    `json:"statusCode"`
	Reason      string `json:"reason,omitempty"`
	Explanation string `json:"explanation"`
	URL         string `json:"url"`
}

func (e *APIError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("codewars api %d (%s): %s [%s]", e.StatusCode, e.Explanation, e.Reason, e.URL)
	}
	return fmt.Sprintf("codewars api %d (%s) [%s]", e.StatusCode, e.Explanation, e.URL)
}

// StatusExplanation maps HTTP status codes documented at https://dev.codewars.com/#errors.
func StatusExplanation(code int) string {
	switch code {
	case 400:
		return "Bad Request -- Something went wrong"
	case 401:
		return "Unauthorized -- Your API key is wrong"
	case 403:
		return "Forbidden -- You do not have permission to access this resource"
	case 404:
		return "Not Found -- The specified resource could not be found"
	case 405:
		return "Method Not Allowed -- You tried to access a resource with an invalid method"
	case 406:
		return "Not Acceptable -- You requested a format that isn't json"
	case 422:
		return "Unprocessable Entity -- Your input failed validation"
	case 429:
		return "Too Many Requests -- You're making too many API requests"
	case 500:
		return "Internal Server Error -- We had a problem with our server. Try again later"
	case 503:
		return "Service Unavailable -- We're temporarily offline for maintenance. Try again later"
	default:
		return fmt.Sprintf("HTTP %d", code)
	}
}

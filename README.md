# codewars-cli (`codewars` / `cw`)

Zero-dependency Go CLI and client library for the [Codewars API v1](https://dev.codewars.com/#introduction), built in pair with [`8b-is/space-bender`](https://github.com/8b-is/space-bender).

## Features

- **Pure Go stdlib (`go 1.24`)** — single static binary, zero external dependencies.
- **Complete Codewars API v1 Coverage (`https://dev.codewars.com/#introduction`)**:
  - **Get User** (`GET /api/v1/users/{user}`) — `codewars user <username_or_id> [--json]`
  - **List Completed Challenges** (`GET /api/v1/users/{user}/code-challenges/completed?page={page}`) — `codewars completed <username_or_id> [--page 0] [--all] [--lang <lang>] [--json]`
  - **List Authored Challenges** (`GET /api/v1/users/{user}/code-challenges/authored`) — `codewars authored <username_or_id> [--json]`
  - **Get Code Challenge** (`GET /api/v1/code-challenges/{challenge}`) — `codewars kata <id_or_slug_or_url> [--desc] [--json]`
  - **Webhooks Receiver & Verifier** (`https://dev.codewars.com/#webhooks`) — `codewars webhook [--addr 127.0.0.1:8795] [--secret <secret>] [--enrich]` (`X-Webhook-Event: webhook | code_challenge | user`, `X-Webhook-Secret` constant-time verification, automatic kata enrichment)
  - **HTTP Error Table** (`https://dev.codewars.com/#errors`) — `codewars errors` (`400`, `401`, `403`, `404`, `405`, `406`, `422`, `429`, `500`, `503`)
- **Paired `8b-is/space-bender` Integration (`codewars bender`)**:
  - `codewars bender list [--live] [--json]` — Lists all katas in `8b-is/space-bender` (`2kyu`, `3kyu`, `6kyu`, `7kyu`, `kumite`) and optionally enriches each kata live from the Codewars API v1.
  - `codewars bender test [--filter <slug_or_tier>] [--json]` — Runs the verification suites across `8b-is/space-bender` (`python3 test_solution.py` and `node test.mjs` against `@codewars/lambda-calculus`).
  - `codewars bender scaffold <id_or_slug_or_url> [--lang python|lc|go] [--slug <name>]` — Fetches a kata from the Codewars API v1 and scaffolds `katas/<kyu>/<slug>/{README.md,solution.<ext>,test_solution.<ext>}` directly inside `space-bender`.

## Install

```bash
go install github.com/8b-is/codewars-cli/cmd/codewars@latest
```

Or from a local checkout:

```bash
cd codewars-cli
go build -o ~/.local/bin/codewars ./cmd/codewars
ln -sf ~/.local/bin/codewars ~/.local/bin/cw
```

## Quick Examples

```bash
# Inspect any kata by slug, ID, URL, or space-bender shorthand
codewars kata evaluate-math
codewars kata valid-braces --desc
codewars kata https://www.codewars.com/kata/52a78825cdfc2cfc87000005 --json

# Query user profile, completed katas, and authored katas
codewars user jhoffner
codewars completed jhoffner --page 0 --lang ruby
codewars authored ALowVerus

# Inspect and run all verification suites in 8b-is/space-bender
codewars bender list --live
codewars bender test

# Scaffold a new kata into space-bender from the Codewars API
codewars bender scaffold valid-braces --lang python
```

## Layout

- `cmd/codewars/main.go` — CLI entrypoint (`codewars` / `cw`)
- `pkg/codewars/types.go` — Codewars API v1 types & HTTP error codes
- `pkg/codewars/client.go` — Pure-stdlib HTTP client (`GetUser`, `ListCompletedChallenges`, `ListAllCompletedChallenges`, `ListAuthoredChallenges`, `GetCodeChallenge`)
- `pkg/codewars/webhook.go` — Codewars Hookbot webhook parser & verifier
- `pkg/bender/bender.go` — `8b-is/space-bender` discovery, live API enrichment, test runner, and kata scaffolder

*the constellation · 0 + 1 · fine touch from within · vaked.dev*

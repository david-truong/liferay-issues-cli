# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`issues` is a single-binary Go CLI that wraps the Atlassian Jira REST API v3 for Liferay's instance. End-user docs live in `README.md`; this file covers internals and dev workflow.

## Common commands

```sh
make build       # go build with version ldflags → ./issues
make install     # go install to $GOPATH/bin
make test        # go test ./...
make lint        # golangci-lint run
make snapshot    # local cross-platform build via GoReleaser (no publish)
make release tag=v1.2.3   # tag, push, and run goreleaser (needs GITHUB_TOKEN)
```

Run a single test:

```sh
go test ./cmd/... -run TestBuildFindJQL
go test ./cmd/... -run 'TestBuildFindJQL/query_with_project'   # subtest
go test -v ./internal/jira/...                                  # one package, verbose
```

The release flow tags `vX.Y.Z`, GoReleaser builds binaries for all platforms and updates the Homebrew tap (`david-truong/homebrew-liferay`).

## Architecture

### Layout

- `main.go` → `cmd.Execute()`.
- `cmd/` — one file per Cobra subcommand (`view`, `create`, `update`, `transition`, `list`, `find`, `jql`, `comment`, `open`, `board`, `sprint`, `config_cmd`). All in `package cmd`. New subcommands must be registered in `cmd/root.go`'s `init()`.
- `internal/config` — viper-backed YAML config at `$XDG_CONFIG_HOME/issues/config.yaml` (default `~/.config/issues/config.yaml`). Holds Jira instance, defaults, and optional auth.
- `internal/jira` — typed REST client (`Client`, `Issue`, `Version`, `Component`, `Sprint`, …). All HTTP goes through `Client.do` / `doWithBase`; the agile API uses `agileBase()` which rewrites `/rest/api/3` → `/rest/agile/1.0`.
- `internal/git` — extracts a ticket key (e.g. `LPD-12345`) from the current git branch. Used as the default ticket when no arg is given.
- `internal/ui` — `tabwriter` tables, ADF text extraction, and `huh`-based interactive prompts.

### Cross-cutting

- **Default subcommand is `view`** (`cmd/root.go:27`). `issues` with no args runs `viewRun` which calls `resolveTicket` → falls back to `internal/git.ExtractTicket()`.
- **Lazy client init.** `cmd/root.go:initClient()` builds the Jira client on first use; subcommands that hit the API must call it. Tests should set the package-level `client` directly or stub via the `cfg` global.
- **Auth resolution order** (`config.ResolveAuth`): `JIRA_USER`/`JIRA_API_TOKEN` env vars → config file → `~/.netrc` for `liferay.atlassian.net`.
- **Globals.** `cmd` has package-level `cfg`, `client`, `authHeader`, `debug`. Tests save/restore `cfg` (see `cmd/find_test.go:31` for the pattern).

### JQL building

Two distinct paths to a JQL string, do not conflate them:

- `cmd/list.go` → `buildFilterJQL` — structured filters (`-p`, `-a`, `--status`, `--board`, `--sprint`).
- `cmd/find.go` → `buildFindJQL` — full-text search (`text ~ "..."`) plus its own filter set (component resolution, fix-version range, `--include-master`).
- `cmd/jql.go` — passes the user's raw query verbatim to `client.Search`. No validation.

`--include-master` requires `--after` or `--before` and resolves version names to release dates via `client.GetProjectVersions`; unresolved version names are an error, not a silent broad match.

### Conventions

- Errors from API calls: wrap with context (`fmt.Errorf("fetching ...: %w", err)`).
- JQL string values: `fmt.Sprintf("field = %q", val)` — Go quoting is good enough for typical input but JQL reserved chars are not escaped.
- Table-driven tests with subtests (`t.Run`); see `cmd/find_test.go` and `internal/jira/client_test.go`.
- The Cobra flag set in `cmd/find_test.go`'s `newFindCmd()` is duplicated from `find.go`'s `init()` — keep them in sync when adding flags.

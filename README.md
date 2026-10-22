# issues

A CLI tool for managing Jira tickets on Liferay's Atlassian instance. Create, view, update, transition, and search issues without leaving the terminal.

## Install

### Homebrew

```sh
brew install david-truong/liferay/liferay-issues-cli
```

The binary installs as `issues`.

### From source

```sh
git clone https://github.com/david-truong/liferay-issues-cli.git
cd liferay-issues-cli
make install
```

### Download binary

Grab the latest release from [GitHub Releases](https://github.com/david-truong/liferay-issues-cli/releases) for your platform.

## Authentication

Configure credentials using one of these methods (checked in order):

### 1. Environment variables

```sh
export JIRA_USER="you@liferay.com"
export JIRA_API_TOKEN="your-api-token"
```

### 2. Config file

```sh
issues config set auth.email you@liferay.com
issues config set auth.token your-api-token
```

### 3. .netrc (legacy)

If you already have a `~/.netrc` entry for Liferay Jira, it works automatically:

```
machine liferay.atlassian.net
login you@liferay.com
password your-api-token
```

Generate an API token at https://id.atlassian.com/manage-profile/security/api-tokens.

## Usage

### View an issue

```sh
# From current git branch (extracts ticket ID like LPD-12345)
issues

# By ticket key (status, assignee, description, attachments)
issues view LPD-12345

# Raw JSON output
issues view LPD-12345 --json

# Extract a specific field
issues view LPD-12345 -f .fields.status.name

# Open in browser
issues open LPD-12345
```

### Create an issue

```sh
# With flags
issues create -p LPD -t Bug -s "Login page crashes on Safari" -d "Steps to reproduce..."

# Interactive mode (prompts for fields)
issues create -i

# Minimal (uses defaults from config)
issues create -s "Fix typo in header"

# Add the new issue to a sprint (ID or name)
issues create -s "Fix typo in header" --sprint "Sprint 42"
```

### Update an issue

```sh
issues update LPD-12345 --summary "Updated title"
issues update LPD-12345 --assignee "Jane Doe"
issues update LPD-12345 --assignee-id ACCOUNT_ID
issues update LPD-12345 --priority High
issues update LPD-12345 --add-label frontend
issues update LPD-12345 --remove-label backend
issues update LPD-12345 --fix-version 7.4.13
issues update LPD-12345 --pull-request https://github.com/liferay/liferay-portal/pull/12345

# Move to a sprint (ID or name), or back to the backlog
issues update LPD-12345 --sprint "Sprint 42"
issues update LPD-12345 --remove-sprint

# Subtasks cannot be in a sprint directly; move the parent without prompting
issues update LPD-12345 --sprint "Sprint 42" --include-parent
```

### Transition an issue

```sh
# Interactive picker
issues transition LPD-12345

# By status name (fuzzy matched)
issues transition LPD-12345 "In Progress"
issues transition LPD-12345 resolve

# With a comment
issues transition LPD-12345 "In Progress" -m "Starting work on this"

# Set fields that the transition screen requires
issues transition LPD-12345 Closed --resolution Fixed
issues transition LPD-12345 "In Peer Review" --pull-request https://github.com/liferay/liferay-portal/pull/12345
```

When the target status is not a direct transition, known LPD and BPR workflows are walked one step at a time (for example, Open → Selected for Development → In Progress).

### List issues

```sh
# My open issues
issues list -a me

# Filter by project and status
issues list -p LPD --status "In Progress"

# Limit results
issues list -a me -n 50

# Issues on a board or in a sprint (ID or name)
issues list --board 998
issues list --sprint "Sprint 42"
```

### Sprints and boards

A sprint name is matched against the active and future sprints on the default board: `jira.default_board` if set, otherwise the scrum boards of `jira.default_project`. An exact name wins; otherwise any sprint whose name contains the text (case insensitive) matches, and you are prompted when more than one does.

```sh
# Active sprints on the default board
issues sprint list

# Other states or boards
issues sprint list --state active,future
issues sprint list --board "Site Management"

# Sprint details
issues sprint view 12345

# Boards
issues board list --type scrum
issues board view 998
```

### Find issues

```sh
# Find issues by text (searches summary, description, comments)
issues find "login"

# Narrow to a project
issues find "login" -p LPD

# Combine with filters
issues find "login" -t Bug -a me

# Filter by component, label, or exact affects version
issues find "login" -c "REST Builder"
issues find "login" -l regression
issues find "login" -v 7.4.13

# Only closed/fixed issues
issues find "login" --fixed

# Filter by status and resolution separately
issues find "login" --status Closed -r Fixed

# Filter by fix version range
issues find "login" --after 7.4.0
issues find "login" --before 7.3.10
issues find "login" --after 7.3.0 --before 7.4.3

# Include tickets with master fix version by creation date
issues find "login" --after 7.4.0 --include-master -p LPD

# Sort results (default is relevance; options: updated, created, priority, key, status, assignee, summary, issuetype, resolution)
issues find "login" --order-by updated

# More results
issues find "login" -n 50
```

### Raw JQL

```sh
# Cross-project query
issues jql 'project in (LPD, LPS) AND assignee = currentUser() AND status != Closed'

# Custom field filter
issues jql 'project = LPD AND "Epic Link" = LPD-12345'
```

### Comments

```sh
# Add a comment
issues comment LPD-12345 -m "This is fixed in the latest build"

# Open your editor to write a comment
issues comment LPD-12345 -e

# Pipe from stdin
echo "Automated comment" | issues comment LPD-12345

# List comments
issues comment LPD-12345 --list

# Upload files, then post the comment (--attach is repeatable)
issues comment LPD-12345 -m "Mockup attached" --attach mockup.png --attach trace.log

# Mention a user by display name (notifies them)
issues comment LPD-12345 -m "@[Jane Doe] can you take a look?"
```

### Attachments

```sh
# Attach one or more files (prints each attachment's id, filename, and size)
issues attach LPD-12345 mockup.png trace.log
```

Attachments are listed in `issues view`. They are attached to the ticket, not embedded inline in a comment.

### Markdown

Comment bodies (including `transition -m`) and `create`/`update` descriptions are written as Markdown and converted to Jira's rich text format: headings, `**bold**`, `*italic*`, `~~strike~~`, `` `code` ``, fenced code blocks (with language), bullet and numbered lists (nested), links, blockquotes, and horizontal rules. Images (`![alt](url)`) become links, and raw HTML is kept as literal text. Plain text works as before: blank lines separate paragraphs and single newlines are line breaks.

Jira's `[~accountid:ID]` mention syntax becomes a real mention everywhere Markdown is accepted. In `issues comment`, `@[Display Name]` is also resolved to that syntax by searching Jira users; an exact (case insensitive) display name match or a single result wins, and otherwise the command fails and lists the candidates with their account IDs.

### Configuration

```sh
# Set default project
issues config set jira.default_project LPD

# Set default issue type
issues config set defaults.issue_type Bug

# Set default board for sprint lookups
issues config set jira.default_board 998

# Set Jira instance (default: liferay.atlassian.net)
issues config set jira.instance your-instance.atlassian.net

# View all settings
issues config list

# Show config file location
issues config path
```

Config file location: `~/.config/issues/config.yaml`

## Shell completions

```sh
# Bash
issues completion bash > /etc/bash_completion.d/issues

# Zsh
issues completion zsh > "${fpath[1]}/_issues"

# Fish
issues completion fish > ~/.config/fish/completions/issues.fish
```

## Development

```sh
make build       # Build binary
make test        # Run tests
make install     # Install to $GOPATH/bin
make snapshot    # Local cross-platform build via GoReleaser
```

### Releasing

```sh
# Tag and publish (requires GITHUB_TOKEN)
make release tag=v1.0.0
```

This uses [GoReleaser](https://goreleaser.com/) to build binaries for all platforms and update the Homebrew tap.

## License

MIT

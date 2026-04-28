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

# By ticket key
issues view LPD-12345

# Detailed view with status, assignee, description
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
```

### Update an issue

```sh
issues update LPD-12345 --summary "Updated title"
issues update LPD-12345 --assignee ACCOUNT_ID
issues update LPD-12345 --priority High
issues update LPD-12345 --add-label frontend
issues update LPD-12345 --remove-label backend
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
```

### List issues

```sh
# My open issues
issues list -a me

# Filter by project and status
issues list -p LPD --status "In Progress"

# Raw JQL
issues list --jql "project = LPD AND assignee = currentUser() ORDER BY updated DESC"

# Limit results
issues list -a me -n 50
```

### Find issues

```sh
# Find issues by text (searches summary, description, comments)
issues find "login"

# Narrow to a project
issues find "login" -p LPD

# Combine with filters
issues find "login" -t Bug -a me

# Filter by component
issues find "login" -c "REST Builder"

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

# More results
issues find "login" -n 50
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
```

### Configuration

```sh
# Set default project
issues config set jira.default_project LPD

# Set default issue type
issues config set defaults.issue_type Bug

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

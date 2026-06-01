# gh-wait-ci

A GitHub CLI extension to wait for CI to complete without polling spam.

## Installation

```bash
gh extension install PazerOP/gh-wait-ci
```

## Usage

```bash
# Wait for CI on current commit
gh wait-ci

# Wait for a specific commit
gh wait-ci --sha c79dcca

# Wait for a specific run by ID
gh wait-ci 12345678

# Wait for CI on a remote repo
gh wait-ci --repo owner/repo

# Wait for a specific commit on a remote repo
gh wait-ci --sha c79dcca --repo owner/repo

# Exit immediately on first failure
gh wait-ci --fail-fast

# Stream each job's logs live as they run (like the GitHub web UI)
gh wait-ci --logs

# Stream logs and poll a little faster
gh wait-ci --logs --interval 3
```

## Flags

| Flag | Description |
| --- | --- |
| `-l`, `--logs` | Stream job logs live as they run, instead of the status summary |
| `-i`, `--interval` | Polling interval in seconds (default `5`) |
| `--fail-fast` | Exit immediately when any job fails |
| `-s`, `--sha` | Commit SHA to watch (full or partial) |
| `-R`, `--repo` | Target repository in `[HOST/]OWNER/REPO` format |

## What it does

1. Checks you're in a git repo with pushed commits
2. Shows repo, branch, commit, and PR info
3. Finds workflow runs for the current commit (retries if not found yet)
4. Uses `gh run watch` to efficiently wait (no polling spam)
5. Reports final status with job details and links
6. Shows failed logs if CI failed

## Requirements

- `gh` CLI authenticated

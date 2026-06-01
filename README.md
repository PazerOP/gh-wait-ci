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

# Watch step-by-step progress live and print each job's log as it finishes
gh wait-ci --logs

# Stream logs and poll a little faster
gh wait-ci --logs --interval 3
```

## Flags

| Flag | Description |
| --- | --- |
| `-l`, `--logs` | Stream step progress live and print each job's full log as it finishes |
| `-i`, `--interval` | Polling interval in seconds (default `5`) |
| `--fail-fast` | Exit immediately when any job fails |
| `-s`, `--sha` | Commit SHA to watch (full or partial) |
| `-R`, `--repo` | Target repository in `[HOST/]OWNER/REPO` format |

## Live logs (`--logs`)

By default `gh wait-ci` shows a live status summary. With `--logs` it also shows
the actual log output:

- **Step progress is live.** As each job runs, its steps print as they start and
  finish (`✓ Build`, `✓ Run tests`, ...), so you can watch progress in the
  terminal instead of refreshing the Actions page.
- **Each job's full log prints the moment that job finishes**, with timestamps
  stripped and `##[group]` / `##[error]` markers cleaned up. In a multi-job
  workflow the logs arrive progressively, one job at a time, as each completes.

### Why logs appear per job, not line-by-line

GitHub's REST API only makes a job's log downloadable once the job has
**completed** — while it runs, the logs endpoint redirects to a storage blob
that doesn't exist yet (HTTP 404). The line-by-line live log you see on
github.com is rendered from the browser's authenticated web session, which a
token-based CLI can't reuse. So `--logs` streams the most granular output the
GitHub API actually exposes to a token: step transitions live, and the full job
log the instant each job finishes.

## What it does

1. Checks you're in a git repo with pushed commits
2. Shows repo, branch, commit, and PR info
3. Finds workflow runs for the current commit (retries if not found yet)
4. Polls run status every few seconds (no `sleep`-loop spam) until everything
   finishes — or, with `--logs`, streams step progress and job logs as above
5. Reports final status with job details and links
6. Shows failed-log commands if CI failed

## Requirements

- `gh` CLI authenticated

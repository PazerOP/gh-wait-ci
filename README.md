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

# Wait for a specific run by ID
gh wait-ci 12345678
```

## What it does

1. Checks you're in a git repo with pushed commits
2. Shows repo, branch, commit, and PR info
3. Finds workflow runs for the current commit (retries if not found yet)
4. Uses `gh run watch` to efficiently wait (no polling spam)
5. Reports final status with job details and links
6. Shows failed logs if CI failed

## Requirements

- `gh` CLI authenticated
- `jq` for JSON parsing

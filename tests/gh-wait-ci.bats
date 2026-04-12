#!/usr/bin/env bats

# Timeout for each test (seconds)
TEST_TIMEOUT=5

setup() {
    # Add mocks to PATH
    export PATH="$BATS_TEST_DIRNAME/mocks:$PATH"
    chmod +x "$BATS_TEST_DIRNAME/mocks/gh" "$BATS_TEST_DIRNAME/mocks/git"
}

# Wrapper with timeout
run_script() {
    timeout "$TEST_TIMEOUT" "$BATS_TEST_DIRNAME/../gh-wait-ci" "$@"
}

@test "shows commands for failed logs instead of inline logs" {
    export MOCK_RUN_LIST_JSON='[{"databaseId": 12345, "status": "completed", "conclusion": "failure", "name": "CI"}]'
    export MOCK_RUN_VIEW_JSON='{
        "status": "completed",
        "conclusion": "failure",
        "name": "CI",
        "url": "https://github.com/test-owner/test-repo/actions/runs/12345",
        "jobs": [
            {"name": "build", "status": "completed", "conclusion": "success", "databaseId": 111},
            {"name": "test", "status": "completed", "conclusion": "failure", "databaseId": 222}
        ]
    }'

    run run_script

    # Should show command hints inline with failed jobs
    [[ "$output" == *"test"*"→"*"gh run view --log --job 222"* ]]
    [[ "$output" == *"View all failed logs"* ]]
    [[ "$output" == *"gh run view 12345 --log-failed"* ]]
}

@test "shows progress percentage" {
    export MOCK_RUN_LIST_JSON='[{"databaseId": 12345, "status": "completed", "conclusion": "success", "name": "CI"}]'
    export MOCK_RUN_VIEW_JSON='{
        "status": "completed",
        "conclusion": "success",
        "name": "CI",
        "url": "https://github.com/test-owner/test-repo/actions/runs/12345",
        "jobs": [
            {"name": "build", "status": "completed", "conclusion": "success", "databaseId": 111},
            {"name": "test", "status": "completed", "conclusion": "success", "databaseId": 222}
        ]
    }'

    run run_script

    # Should show progress with percentage
    [[ "$output" == *"Progress: 2/2 (100%)"* ]]
}

@test "shows all job statuses" {
    export MOCK_RUN_LIST_JSON='[{"databaseId": 12345, "status": "completed", "conclusion": "success", "name": "CI"}]'

    export MOCK_RUN_VIEW_JSON='{
        "status": "completed",
        "conclusion": "success",
        "name": "CI",
        "url": "https://github.com/test-owner/test-repo/actions/runs/12345",
        "jobs": [
            {"name": "build", "status": "completed", "conclusion": "success", "databaseId": 111},
            {"name": "test", "status": "completed", "conclusion": "success", "databaseId": 222},
            {"name": "lint", "status": "completed", "conclusion": "skipped", "databaseId": 333}
        ]
    }'

    run run_script

    # Should show all jobs with their statuses
    [[ "$output" == *"build"* ]]
    [[ "$output" == *"test"* ]]
    [[ "$output" == *"lint"* ]]
    [[ "$output" == *"skipped"* ]]
}

@test "successful run shows PASSED" {
    export MOCK_RUN_LIST_JSON='[{"databaseId": 12345, "status": "completed", "conclusion": "success", "name": "CI"}]'
    export MOCK_RUN_VIEW_JSON='{
        "status": "completed",
        "conclusion": "success",
        "name": "CI",
        "url": "https://github.com/test-owner/test-repo/actions/runs/12345",
        "jobs": [{"name": "build", "status": "completed", "conclusion": "success", "databaseId": 111}]
    }'

    run run_script

    [[ "$output" == *"PASSED"* ]]
    [[ "$status" -eq 0 ]]
}

@test "failed run shows FAILED and exits non-zero" {
    export MOCK_RUN_LIST_JSON='[{"databaseId": 12345, "status": "completed", "conclusion": "failure", "name": "CI"}]'
    export MOCK_RUN_VIEW_JSON='{
        "status": "completed",
        "conclusion": "failure",
        "name": "CI",
        "url": "https://github.com/test-owner/test-repo/actions/runs/12345",
        "jobs": [{"name": "build", "status": "completed", "conclusion": "failure", "databaseId": 111}]
    }'

    run run_script

    [[ "$output" == *"FAILED"* ]]
    [[ "$status" -ne 0 ]]
}

@test "--repo flag targets specified repository" {
    export MOCK_RUN_LIST_JSON='[{"databaseId": 99999, "status": "completed", "conclusion": "success", "name": "CI"}]'
    export MOCK_RUN_VIEW_JSON='{
        "status": "completed",
        "conclusion": "success",
        "name": "CI",
        "url": "https://github.com/other-owner/other-repo/actions/runs/99999",
        "jobs": [{"name": "build", "status": "completed", "conclusion": "success", "databaseId": 111}]
    }'
    export MOCK_API_REPO_JSON='{"default_branch": "main"}'
    export MOCK_API_COMMITS_JSON='{"sha": "def456abc789012"}'

    run run_script --repo other-owner/other-repo

    [[ "$output" == *"other-owner/other-repo"* ]]
    [[ "$output" == *"PASSED"* ]]
    [[ "$status" -eq 0 ]]
}

@test "-R short flag works same as --repo" {
    export MOCK_RUN_LIST_JSON='[{"databaseId": 99999, "status": "completed", "conclusion": "success", "name": "CI"}]'
    export MOCK_RUN_VIEW_JSON='{
        "status": "completed",
        "conclusion": "success",
        "name": "CI",
        "url": "https://github.com/other-owner/other-repo/actions/runs/99999",
        "jobs": [{"name": "build", "status": "completed", "conclusion": "success", "databaseId": 111}]
    }'
    export MOCK_API_REPO_JSON='{"default_branch": "main"}'
    export MOCK_API_COMMITS_JSON='{"sha": "def456abc789012"}'

    run run_script -R other-owner/other-repo

    [[ "$output" == *"other-owner/other-repo"* ]]
    [[ "$output" == *"PASSED"* ]]
    [[ "$status" -eq 0 ]]
}

@test "--repo flag with run-id targets specified repository" {
    export MOCK_RUN_VIEW_JSON='{
        "status": "completed",
        "conclusion": "success",
        "name": "CI",
        "url": "https://github.com/other-owner/other-repo/actions/runs/55555",
        "jobs": [{"name": "build", "status": "completed", "conclusion": "success", "databaseId": 111}]
    }'

    run run_script --repo other-owner/other-repo 55555

    [[ "$output" == *"other-owner/other-repo"* ]]
    [[ "$output" == *"Watching specified run: 55555"* ]]
    [[ "$output" == *"PASSED"* ]]
    [[ "$status" -eq 0 ]]
}

@test "shows commit and PR links" {
    export MOCK_RUN_LIST_JSON='[{"databaseId": 12345, "status": "completed", "conclusion": "success", "name": "CI"}]'
    export MOCK_RUN_VIEW_JSON='{
        "status": "completed",
        "conclusion": "success",
        "name": "CI",
        "url": "https://github.com/test-owner/test-repo/actions/runs/12345",
        "jobs": [{"name": "build", "status": "completed", "conclusion": "success", "databaseId": 111}]
    }'

    run run_script

    [[ "$output" == *"Commit:"* ]]
    [[ "$output" == *"PR:"* ]]
    [[ "$output" == *"github.com"* ]]
}

@test "auto-discovers single git repo in subdirectory" {
    export MOCK_GIT_NOT_IN_REPO=true
    export MOCK_RUN_LIST_JSON='[{"databaseId": 12345, "status": "completed", "conclusion": "success", "name": "CI"}]'
    export MOCK_RUN_VIEW_JSON='{
        "status": "completed",
        "conclusion": "success",
        "name": "CI",
        "url": "https://github.com/test-owner/test-repo/actions/runs/12345",
        "jobs": [{"name": "build", "status": "completed", "conclusion": "success", "databaseId": 111}]
    }'

    TMPDIR=$(mktemp -d)
    mkdir -p "$TMPDIR/my-project/.git"

    cd "$TMPDIR"
    run "$BATS_TEST_DIRNAME/../gh-wait-ci"

    [[ "$output" == *"Found git repository in ./my-project"* ]]
    [[ "$output" == *"PASSED"* ]]
    [[ "$status" -eq 0 ]]

    rm -rf "$TMPDIR"
}

@test "errors with list when multiple git repos found in subdirectories" {
    export MOCK_GIT_NOT_IN_REPO=true

    TMPDIR=$(mktemp -d)
    mkdir -p "$TMPDIR/repo-a/.git"
    mkdir -p "$TMPDIR/repo-b/.git"

    cd "$TMPDIR"
    run "$BATS_TEST_DIRNAME/../gh-wait-ci"

    [[ "$output" == *"multiple repositories"* ]]
    [[ "$output" == *"repo-a"* ]]
    [[ "$output" == *"repo-b"* ]]
    [[ "$status" -ne 0 ]]

    rm -rf "$TMPDIR"
}

@test "errors when not in git repo and no subdirectory repos found" {
    export MOCK_GIT_NOT_IN_REPO=true

    TMPDIR=$(mktemp -d)

    cd "$TMPDIR"
    run "$BATS_TEST_DIRNAME/../gh-wait-ci"

    [[ "$output" == *"not in a git repository"* ]]
    [[ "$status" -ne 0 ]]

    rm -rf "$TMPDIR"
}

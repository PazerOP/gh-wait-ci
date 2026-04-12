package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[1;33m"
	colorBlue   = "\033[0;34m"
	colorReset  = "\033[0m"
)

func printError(msg string) {
	fmt.Fprintf(os.Stderr, "%sERROR: %s%s\n", colorRed, msg, colorReset)
}

func printInfo(msg string) {
	fmt.Printf("%s%s%s\n", colorBlue, msg, colorReset)
}

func printSuccess(msg string) {
	fmt.Printf("%s%s%s\n", colorGreen, msg, colorReset)
}

func printWarn(msg string) {
	fmt.Printf("%s%s%s\n", colorYellow, msg, colorReset)
}

type Context struct {
	Commit      string
	ShortCommit string
	Branch      string
	Repo        string
	CommitURL   string
	PRURL       string
	PRNum       string
}

type RunInfo struct {
	DatabaseID int    `json:"databaseId"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Name       string `json:"name"`
}

type Job struct {
	DatabaseID int    `json:"databaseId"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

type RunDetail struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Jobs       []Job  `json:"jobs"`
}

type PRInfo struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

type RepoInfo struct {
	NameWithOwner string `json:"nameWithOwner"`
}

type RemoteRepoInfo struct {
	DefaultBranch string `json:"default_branch"`
}

type RemoteCommitInfo struct {
	SHA string `json:"sha"`
}

// repoFlag holds the --repo/-R flag value. When set, it overrides repo detection
// and causes gh commands to target the specified repository.
var repoFlag string

// ghCommand runs a gh CLI command, automatically injecting -R <repo> when repoFlag is set.
func ghCommand(args ...string) (string, error) {
	if repoFlag != "" {
		args = append([]string{"-R", repoFlag}, args...)
	}
	return runCommand("gh", args...)
}

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// findGitRepo checks if the current directory is inside a git repository.
// If not, it searches immediate subdirectories for git repos and changes
// into the directory if exactly one is found.
func findGitRepo() error {
	_, err := runCommand("git", "rev-parse", "--git-dir")
	if err == nil {
		return nil
	}

	// Not in a git repo — search immediate subdirectories
	entries, err := os.ReadDir(".")
	if err != nil {
		return fmt.Errorf("not in a git repository")
	}

	var repos []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if _, statErr := os.Stat(entry.Name() + "/.git"); statErr == nil {
			repos = append(repos, entry.Name())
		}
	}

	switch len(repos) {
	case 0:
		return fmt.Errorf("not in a git repository")
	case 1:
		printInfo(fmt.Sprintf("Found git repository in ./%s, using it", repos[0]))
		if err := os.Chdir(repos[0]); err != nil {
			return fmt.Errorf("could not enter repository %s: %w", repos[0], err)
		}
		return nil
	default:
		return fmt.Errorf("not in a git repository, found multiple repositories: %s\nPlease cd into one or use --repo/-R", strings.Join(repos, ", "))
	}
}

// checkPushed checks if there are unpushed commits. Returns the commit to use
// (either HEAD if all pushed, or the upstream commit if unpushed commits exist)
// and a boolean indicating if we fell back to the upstream commit.
func checkPushed() (string, bool, error) {
	unpushed, err := runCommand("git", "log", "@{u}..HEAD", "--oneline")
	if err != nil {
		// If there's no upstream, use HEAD (can't determine pushed state)
		return "HEAD", false, nil
	}
	if unpushed != "" {
		// Get the upstream commit to use instead
		upstreamCommit, err := runCommand("git", "rev-parse", "@{u}")
		if err != nil {
			return "", false, fmt.Errorf("could not get upstream commit: %w", err)
		}
		return upstreamCommit, true, nil
	}
	return "HEAD", false, nil
}

func getContext(commitRef string) (*Context, error) {
	ctx := &Context{}

	if repoFlag != "" {
		return getRemoteContext()
	}

	commit, err := runCommand("git", "rev-parse", commitRef)
	if err != nil {
		return nil, fmt.Errorf("could not get commit: %w", err)
	}
	ctx.Commit = commit

	shortCommit, err := runCommand("git", "rev-parse", "--short", commitRef)
	if err != nil {
		return nil, fmt.Errorf("could not get short commit: %w", err)
	}
	ctx.ShortCommit = shortCommit

	branch, err := runCommand("git", "branch", "--show-current")
	if err != nil {
		return nil, fmt.Errorf("could not get branch: %w", err)
	}
	ctx.Branch = branch

	repoJSON, err := runCommand("gh", "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		return nil, fmt.Errorf("could not determine GitHub repository")
	}

	var repoInfo RepoInfo
	if err := json.Unmarshal([]byte(repoJSON), &repoInfo); err != nil {
		return nil, fmt.Errorf("could not parse repo info: %w", err)
	}
	ctx.Repo = repoInfo.NameWithOwner

	ctx.CommitURL = fmt.Sprintf("https://github.com/%s/commit/%s", ctx.Repo, ctx.Commit)

	return ctx, nil
}

// getRemoteContext builds a Context by querying the remote repository specified by repoFlag.
func getRemoteContext() (*Context, error) {
	ctx := &Context{}
	ctx.Repo = repoFlag

	// Get the default branch of the remote repo
	repoJSON, err := runCommand("gh", "api", fmt.Sprintf("repos/%s", repoFlag))
	if err != nil {
		return nil, fmt.Errorf("could not query repository %s: %w", repoFlag, err)
	}

	var remoteRepo RemoteRepoInfo
	if err := json.Unmarshal([]byte(repoJSON), &remoteRepo); err != nil {
		return nil, fmt.Errorf("could not parse repo info: %w", err)
	}
	ctx.Branch = remoteRepo.DefaultBranch

	// Get the latest commit on the default branch
	commitJSON, err := runCommand("gh", "api", fmt.Sprintf("repos/%s/commits/%s", repoFlag, ctx.Branch))
	if err != nil {
		return nil, fmt.Errorf("could not get latest commit for %s: %w", ctx.Branch, err)
	}

	var commitInfo RemoteCommitInfo
	if err := json.Unmarshal([]byte(commitJSON), &commitInfo); err != nil {
		return nil, fmt.Errorf("could not parse commit info: %w", err)
	}
	ctx.Commit = commitInfo.SHA
	if len(ctx.Commit) >= 7 {
		ctx.ShortCommit = ctx.Commit[:7]
	} else {
		ctx.ShortCommit = ctx.Commit
	}

	ctx.CommitURL = fmt.Sprintf("https://github.com/%s/commit/%s", ctx.Repo, ctx.Commit)

	return ctx, nil
}

func printContext(ctx *Context) {
	printInfo(fmt.Sprintf("Repository: %s", ctx.Repo))
	printInfo(fmt.Sprintf("Branch: %s", ctx.Branch))
	printInfo(fmt.Sprintf("Commit: %s", ctx.ShortCommit))
	fmt.Println()
}

func getPRInfo(ctx *Context) {
	prJSON, err := ghCommand("pr", "view", "--json", "number,url")
	if err != nil {
		return
	}

	var prInfo PRInfo
	if err := json.Unmarshal([]byte(prJSON), &prInfo); err != nil {
		return
	}

	ctx.PRNum = strconv.Itoa(prInfo.Number)
	ctx.PRURL = prInfo.URL
}

func findRuns(ctx *Context, runID string) ([]int, error) {
	if runID != "" {
		id, err := strconv.Atoi(runID)
		if err != nil {
			return nil, fmt.Errorf("invalid run ID: %s", runID)
		}
		printInfo(fmt.Sprintf("Watching specified run: %s", runID))
		return []int{id}, nil
	}

	printInfo(fmt.Sprintf("Finding workflow runs for commit %s...", ctx.ShortCommit))

	var runs []RunInfo
	for i := 1; i <= 5; i++ {
		runsJSON, err := ghCommand("run", "list", "--commit", ctx.Commit,
			"--json", "databaseId,status,conclusion,name", "--limit", "10")
		if err != nil {
			runsJSON = "[]"
		}

		if err := json.Unmarshal([]byte(runsJSON), &runs); err != nil {
			runs = []RunInfo{}
		}

		if len(runs) > 0 {
			break
		}

		if i < 5 {
			printWarn(fmt.Sprintf("No runs found yet, waiting 5 seconds... (attempt %d/5)", i))
			time.Sleep(5 * time.Second)
		}
	}

	if len(runs) == 0 {
		return nil, fmt.Errorf("no workflow runs found for commit %s", ctx.ShortCommit)
	}

	runIDs := make([]int, len(runs))
	printInfo(fmt.Sprintf("Found %d workflow run(s):", len(runs)))
	for i, run := range runs {
		runIDs[i] = run.DatabaseID
		fmt.Printf("  %d %s\n", run.DatabaseID, run.Name)
	}
	fmt.Println()

	return runIDs, nil
}

func getRunDetail(runID int) (*RunDetail, error) {
	runJSON, err := ghCommand("run", "view", strconv.Itoa(runID),
		"--json", "status,conclusion,name,jobs,url")
	if err != nil {
		return nil, err
	}

	var detail RunDetail
	if err := json.Unmarshal([]byte(runJSON), &detail); err != nil {
		return nil, err
	}

	return &detail, nil
}

// waitForRuns waits for all runs to complete. If failFast is true, returns immediately
// when any job fails. Returns (hasFailure, error).
func waitForRuns(runIDs []int, failFast bool) (bool, error) {
	printInfo("Waiting for all runs to complete...")
	fmt.Println()

	lastState := ""
	firstPrint := true
	hasFailure := false

	for {
		allDone := true
		totalJobs := 0
		completedJobs := 0
		currentState := ""
		var output strings.Builder

		for _, runID := range runIDs {
			detail, err := getRunDetail(runID)
			if err != nil {
				continue
			}

			for _, job := range detail.Jobs {
				totalJobs++
				currentState += fmt.Sprintf("%d:%s:%s:%s|", runID, job.Name, job.Status, job.Conclusion)

				var line string
				if job.Status == "completed" {
					completedJobs++
					switch job.Conclusion {
					case "success":
						line = fmt.Sprintf("  ✅ %s / %s\n", detail.Name, job.Name)
					case "skipped":
						line = fmt.Sprintf("  ⏭️  %s / %s (skipped)\n", detail.Name, job.Name)
					case "cancelled":
						line = fmt.Sprintf("  ⛔ %s / %s (cancelled)\n", detail.Name, job.Name)
						hasFailure = true
					default:
						line = fmt.Sprintf("  ❌ %s / %s (%s)\n", detail.Name, job.Name, job.Conclusion)
						hasFailure = true
					}
				} else if job.Status == "in_progress" {
					line = fmt.Sprintf("  🔄 %s / %s\n", detail.Name, job.Name)
				} else if job.Status == "queued" || job.Status == "waiting" {
					line = fmt.Sprintf("  ⏳ %s / %s\n", detail.Name, job.Name)
				} else {
					line = fmt.Sprintf("  ⏳ %s / %s (%s)\n", detail.Name, job.Name, job.Status)
				}
				output.WriteString(line)
			}

			if detail.Status != "completed" {
				allDone = false
			}
		}

		percent := 0
		if totalJobs > 0 {
			percent = completedJobs * 100 / totalJobs
		}

		if currentState != lastState {
			if !firstPrint {
				linesToClear := totalJobs + 1
				for i := 0; i < linesToClear; i++ {
					fmt.Print("\033[A\033[2K")
				}
			}
			firstPrint = false

			printInfo(fmt.Sprintf("Progress: %d/%d (%d%%)", completedJobs, totalJobs, percent))
			fmt.Print(output.String())
			lastState = currentState
		}

		if failFast && hasFailure {
			fmt.Println()
			printWarn("Failure detected, exiting early (--fail-fast)")
			return true, nil
		}

		if allDone {
			break
		}

		time.Sleep(5 * time.Second)
	}
	fmt.Println()

	return hasFailure, nil
}

func showResults(runIDs []int, ctx *Context) bool {
	allSuccess := true

	for _, runID := range runIDs {
		detail, err := getRunDetail(runID)
		if err != nil {
			printError(fmt.Sprintf("Could not get run details for %d", runID))
			continue
		}

		fmt.Println("════════════════════════════════════════════════════════════════")
		if detail.Conclusion == "success" {
			printSuccess(fmt.Sprintf("✅ %s PASSED", detail.Name))
		} else {
			printError(fmt.Sprintf("❌ %s FAILED", detail.Name))
			allSuccess = false
		}
		fmt.Println("════════════════════════════════════════════════════════════════")
		fmt.Println()

		printInfo("Jobs:")
		for _, job := range detail.Jobs {
			var icon string
			switch job.Conclusion {
			case "success":
				icon = "✅"
			case "failure":
				icon = "❌"
			case "cancelled":
				icon = "⛔"
			case "skipped":
				icon = "⏭️ "
			default:
				icon = "⏳"
			}

			if job.Conclusion == "failure" {
				fmt.Printf("  %s %s  →  gh run view --log --job %d\n", icon, job.Name, job.DatabaseID)
			} else {
				fmt.Printf("  %s %s\n", icon, job.Name)
			}
		}
		fmt.Println()

		fmt.Printf("     Run:  %s\n", detail.URL)

		if detail.Conclusion != "success" {
			fmt.Println()
			printWarn("View all failed logs:")
			fmt.Printf("  gh run view %d --log-failed\n", runID)
			fmt.Println()
		}
	}

	printInfo("Links:")
	fmt.Printf("  Commit:  %s\n", ctx.CommitURL)
	if ctx.PRURL != "" {
		fmt.Printf("      PR:  %s\n", ctx.PRURL)
	}
	fmt.Println()

	return allSuccess
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "gh-wait-ci [run-id]",
		Short: "Wait for GitHub Actions CI to complete and report results",
		Long:  "Wait for GitHub Actions CI to complete and report results.\nIf no run-id provided, waits for ALL runs for the current commit.",
		Args:  cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:  run,
	}

	rootCmd.Flags().BoolP("fail-fast", "", false, "Exit immediately when any job fails")
	rootCmd.Flags().StringVarP(&repoFlag, "repo", "R", "", "Target repository in [HOST/]OWNER/REPO format")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	failFast, _ := cmd.Flags().GetBool("fail-fast")

	// When --repo is not set, we need to be in a git repository.
	// If we're not in one, try to find one in a subdirectory.
	if repoFlag == "" {
		if err := findGitRepo(); err != nil {
			printError(err.Error())
			return err
		}
	}

	runID := ""
	if len(args) > 0 {
		runID = args[0]
	}

	// Determine which commit to use
	commitRef := "HEAD"
	if runID == "" && repoFlag == "" {
		// Only check for unpushed commits when auto-detecting runs from local repo
		ref, usedUpstream, err := checkPushed()
		if err != nil {
			printError(err.Error())
			return err
		}
		if usedUpstream {
			printWarn("Warning: You have unpushed commits. Watching the latest pushed commit instead.")
			fmt.Println()
		}
		commitRef = ref
	}

	ctx, err := getContext(commitRef)
	if err != nil {
		printError(err.Error())
		return err
	}

	printContext(ctx)
	getPRInfo(ctx)

	runIDs, err := findRuns(ctx, runID)
	if err != nil {
		printError(err.Error())
		return err
	}

	hasFailure, err := waitForRuns(runIDs, failFast)
	if err != nil {
		printError(err.Error())
		return err
	}

	if failFast && hasFailure {
		showResults(runIDs, ctx)
		return fmt.Errorf("CI failed")
	}

	if !showResults(runIDs, ctx) {
		return fmt.Errorf("CI failed")
	}

	return nil
}

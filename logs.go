package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// tsRe matches the leading RFC3339Nano timestamp that GitHub prefixes onto each
// raw job-log line, e.g. "2026-06-01T02:05:01.1234567Z ". We strip it so the
// streamed output reads like the live log view on github.com.
var tsRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d+Z `)

// jobLogState tracks how much of a job's log has already been printed so each
// poll only emits the newly-appended lines.
type jobLogState struct {
	label    string
	printed  int  // byte offset (always at a line boundary) already emitted
	finished bool // final delta + conclusion banner printed
}

// jobRef pairs a job with the name of the run it belongs to.
type jobRef struct {
	runName string
	job     Job
}

// fetchJobLog downloads the plain-text log for a single job via the REST API.
// While a job is in progress GitHub serves the logs accumulated so far; once the
// job completes it serves the full log. When no log is available yet the API
// responds with an error, which the caller treats as "nothing to show yet".
//
// We deliberately do not use runCommand here because it trims surrounding
// whitespace, which would corrupt the byte-offset bookkeeping used for deltas.
func fetchJobLog(repo string, jobID int) (string, error) {
	cmd := exec.Command("gh", "api", fmt.Sprintf("repos/%s/actions/jobs/%d/logs", repo, jobID))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// cleanLogLine strips the leading timestamp and normalizes GitHub workflow
// command markers (##[group], ##[endgroup], ##[error], ...) into something
// readable in a terminal. The second return value is false when the line should
// be dropped entirely (e.g. ##[endgroup]).
func cleanLogLine(line string) (string, bool) {
	line = strings.TrimRight(line, "\r")
	line = tsRe.ReplaceAllString(line, "")

	switch {
	case strings.HasPrefix(line, "##[group]"):
		return "‣ " + strings.TrimPrefix(line, "##[group]"), true
	case strings.HasPrefix(line, "##[endgroup]"):
		return "", false
	case strings.HasPrefix(line, "##[error]"):
		return colorRed + strings.TrimPrefix(line, "##[error]") + colorReset, true
	case strings.HasPrefix(line, "##[warning]"):
		return colorYellow + strings.TrimPrefix(line, "##[warning]") + colorReset, true
	case strings.HasPrefix(line, "##[notice]"):
		return strings.TrimPrefix(line, "##[notice]"), true
	case strings.HasPrefix(line, "##[command]"):
		return colorBlue + strings.TrimPrefix(line, "##[command]") + colorReset, true
	case strings.HasPrefix(line, "##[section]"):
		return strings.TrimPrefix(line, "##[section]"), true
	}
	return line, true
}

// printLogLine prints a single cleaned log line, optionally prefixed with the
// job label so concurrent jobs can be told apart.
func printLogLine(label, line string, prefix bool) {
	cleaned, ok := cleanLogLine(line)
	if !ok {
		return
	}
	if prefix {
		fmt.Printf("%s%s%s │ %s\n", colorBlue, label, colorReset, cleaned)
	} else {
		fmt.Println(cleaned)
	}
}

// emitDelta prints the portion of raw that has not been printed yet. When final
// is false only whole lines (through the last newline) are emitted so a partial
// trailing line is never shown; on the final flush everything remaining is
// printed. raw is assumed to be append-only across polls, which is how GitHub
// serves growing in-progress logs.
func emitDelta(st *jobLogState, raw string, prefix, final bool) {
	if len(raw) < st.printed {
		// Shouldn't happen (logs only grow); guard against a short read.
		return
	}
	chunk := raw[st.printed:]
	if !final {
		nl := strings.LastIndexByte(chunk, '\n')
		if nl < 0 {
			return // no complete new line yet
		}
		chunk = chunk[:nl+1]
	}
	if chunk == "" {
		return
	}
	st.printed += len(chunk)
	for _, line := range strings.Split(strings.TrimRight(chunk, "\n"), "\n") {
		printLogLine(st.label, line, prefix)
	}
}

// printJobBanner announces a job starting or finishing.
func printJobBanner(label, state string) {
	switch state {
	case "started":
		fmt.Printf("\n%s▶ %s%s\n", colorBlue, label, colorReset)
	case "success":
		fmt.Printf("%s✅ %s — passed%s\n", colorGreen, label, colorReset)
	case "skipped":
		fmt.Printf("%s⏭️  %s — skipped%s\n", colorYellow, label, colorReset)
	case "cancelled":
		fmt.Printf("%s⛔ %s — cancelled%s\n", colorRed, label, colorReset)
	default:
		fmt.Printf("%s❌ %s — %s%s\n", colorRed, label, state, colorReset)
	}
}

// streamLogs waits for all runs to finish while streaming each job's log output
// as GitHub makes it available. It returns whether any job failed.
func streamLogs(runIDs []int, ctx *Context, failFast bool, interval time.Duration) (bool, error) {
	printInfo("Streaming live logs (output appears as GitHub publishes it)...")

	states := map[int]*jobLogState{}
	hasFailure := false

	for {
		allDone := true
		var jobs []jobRef

		for _, runID := range runIDs {
			detail, err := getRunDetail(runID)
			if err != nil {
				allDone = false
				continue
			}
			if detail.Status != "completed" {
				allDone = false
			}
			for _, job := range detail.Jobs {
				jobs = append(jobs, jobRef{runName: detail.Name, job: job})
			}
		}

		prefix := len(jobs) > 1

		for _, jr := range jobs {
			job := jr.job

			// No logs exist until a job actually starts running.
			if job.Status != "in_progress" && job.Status != "completed" {
				continue
			}

			st := states[job.DatabaseID]
			if st == nil {
				label := job.Name
				if prefix {
					label = jr.runName + " / " + job.Name
				}
				st = &jobLogState{label: label}
				states[job.DatabaseID] = st
				printJobBanner(label, "started")
			}
			if st.finished {
				continue
			}

			completed := job.Status == "completed"
			if raw, err := fetchJobLog(ctx.Repo, job.DatabaseID); err == nil {
				emitDelta(st, raw, prefix, completed)
			}

			if completed {
				st.finished = true
				switch job.Conclusion {
				case "success", "skipped", "neutral":
				default:
					hasFailure = true
				}
				printJobBanner(st.label, job.Conclusion)
			}
		}

		if failFast && hasFailure {
			fmt.Println()
			printWarn("Failure detected, exiting early (--fail-fast)")
			return true, nil
		}
		if allDone {
			break
		}
		time.Sleep(interval)
	}

	fmt.Println()
	return hasFailure, nil
}

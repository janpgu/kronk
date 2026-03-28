// Package worker handles job execution as subprocesses.
package worker

import (
	"bytes"
	"database/sql"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/job"
	"github.com/janpgu/kronk/internal/scheduler"
)

// Execute runs a job as a subprocess, records the result, and updates
// the job's next_run_at. If the job is already running, it is skipped.
func Execute(database *sql.DB, j *job.Job, verbose bool) error {
	// Concurrency guard — skip if already running.
	running, err := db.HasRunningInstance(database, j.ID)
	if err != nil {
		return fmt.Errorf("could not check running status for %q: %w", j.Name, err)
	}
	if running {
		if verbose {
			fmt.Printf("skipping %q — already running\n", j.Name)
		}
		return nil
	}

	// Record the start of this run.
	attempt := lastAttempt(database, j.ID) + 1
	run := &job.Run{
		JobID:     j.ID,
		StartedAt: time.Now(),
		Attempt:   attempt,
	}
	runID, err := db.AddRun(database, run)
	if err != nil {
		return fmt.Errorf("could not record run start for %q: %w", j.Name, err)
	}
	run.ID = runID

	if verbose {
		fmt.Printf("running %q (attempt %d): %s\n", j.Name, attempt, j.Command)
	}

	// Execute the command as a subprocess.
	stdout, stderr, exitCode := runCommand(j.Command)
	now := time.Now()
	run.FinishedAt = &now
	run.ExitCode = &exitCode
	run.Stdout = stdout
	run.Stderr = stderr

	if err := db.UpdateRun(database, run); err != nil {
		return fmt.Errorf("could not save run result for %q: %w", j.Name, err)
	}

	if exitCode == 0 {
		// Success — reset attempt counter and schedule next run.
		if verbose {
			fmt.Printf("  ✓ %q finished OK\n", j.Name)
		}
		j.LastRunAt = &now
		nextRun, err := scheduler.NextRun(j.ScheduleCron, now)
		if err != nil {
			return fmt.Errorf("could not compute next run for %q: %w", j.Name, err)
		}
		j.NextRunAt = &nextRun
		j.Status = job.StatusActive
	} else {
		// Failure — retry with exponential backoff or mark failed.
		if verbose {
			fmt.Printf("  ✗ %q failed with exit code %d\n", j.Name, exitCode)
		}
		if attempt <= j.MaxRetries {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			nextRun := now.Add(backoff)
			j.NextRunAt = &nextRun
			j.Status = job.StatusActive
			if verbose {
				fmt.Printf("    retrying in %s (attempt %d/%d)\n", backoff, attempt, j.MaxRetries)
			}
		} else {
			j.Status = job.StatusFailed
			j.LastRunAt = &now
			if verbose && j.MaxRetries > 0 {
				fmt.Printf("    no retries remaining — job marked as failed\n")
			}
		}
	}

	return db.UpdateJob(database, j)
}

// runCommand executes a shell command and returns stdout, stderr, and exit code.
// Uses "sh -c" on Unix and "cmd /C" on Windows.
func runCommand(command string) (stdout, stderr string, exitCode int) {
	var stdoutBuf, stderrBuf bytes.Buffer

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	// Normalise Windows line endings (\r\n → \n) so \r does not corrupt terminal output.
	stdout = strings.ReplaceAll(stdoutBuf.String(), "\r\n", "\n")
	stderr = strings.ReplaceAll(stderrBuf.String(), "\r\n", "\n")

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdout, stderr, exitErr.ExitCode()
		}
		// Command could not be started at all (e.g. binary not found).
		return stdout, err.Error(), 1
	}
	return stdout, stderr, 0
}

// lastAttempt returns the highest attempt number for a job's runs, or 0 if none.
func lastAttempt(database *sql.DB, jobID int64) int {
	runs, err := db.GetRunsForJob(database, jobID, 1)
	if err != nil || len(runs) == 0 {
		return 0
	}
	return runs[0].Attempt
}

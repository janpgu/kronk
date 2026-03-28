// Package job defines the core data types for kronk.
package job

import "time"

// Config holds runtime configuration passed through all commands.
// It is populated in the root command's PersistentPreRunE and threaded
// into every subcommand — there is no global state.
type Config struct {
	DBPath string
}

// Status represents the current state of a job.
type Status string

const (
	StatusActive  Status = "active"
	StatusRunning Status = "running"
	StatusFailed  Status = "failed"
	StatusPaused  Status = "paused"
)

// Job represents a scheduled task stored in the jobs table.
type Job struct {
	ID          int64
	Name        string
	Command     string
	ScheduleRaw string    // human-readable input, e.g. "every night"
	ScheduleCron string   // resolved cron expression, e.g. "0 2 * * *"
	MaxRetries  int
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
	LastRunAt   *time.Time // nil if never run
	NextRunAt   *time.Time // nil if not scheduled
}

// Run represents a single execution of a job, stored in the runs table.
type Run struct {
	ID         int64
	JobID      int64
	StartedAt  time.Time
	FinishedAt *time.Time // nil if still running
	ExitCode   *int       // nil if still running
	Stdout     string
	Stderr     string
	Attempt    int
}

// Duration returns how long the run took. Returns zero if not finished.
func (r *Run) Duration() time.Duration {
	if r.FinishedAt == nil {
		return 0
	}
	return r.FinishedAt.Sub(r.StartedAt)
}

// Succeeded reports whether the run completed with exit code 0.
func (r *Run) Succeeded() bool {
	return r.ExitCode != nil && *r.ExitCode == 0
}

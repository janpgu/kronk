package worker

import (
	"database/sql"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/job"
	_ "modernc.org/sqlite"
)

// openTestDB creates an in-memory SQLite database for testing.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open(':memory:') unexpected error: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("Migrate() unexpected error: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// addJob inserts a job and returns it with its assigned ID.
func addJob(t *testing.T, database *sql.DB, j *job.Job) *job.Job {
	t.Helper()
	id, err := db.AddJob(database, j)
	if err != nil {
		t.Fatalf("AddJob(%q) unexpected error: %v", j.Name, err)
	}
	j.ID = id
	return j
}

// sampleJob returns an active job with a valid schedule.
func sampleJob(name, command string) *job.Job {
	next := time.Now().Add(-time.Minute)
	return &job.Job{
		Name:         name,
		Command:      command,
		ScheduleRaw:  "every minute",
		ScheduleCron: "* * * * *",
		MaxRetries:   0,
		Status:       job.StatusActive,
		NextRunAt:    &next,
	}
}

func TestExecute_Success(t *testing.T) {
	database := openTestDB(t)
	j := addJob(t, database, sampleJob("hello", "echo hello"))

	if err := Execute(database, j, false); err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Job should be active with an updated next run time.
	got, err := db.GetJob(database, "hello")
	if err != nil {
		t.Fatalf("GetJob() unexpected error: %v", err)
	}
	if got.Status != job.StatusActive {
		t.Errorf("Execute() status = %q, want %q", got.Status, job.StatusActive)
	}
	if got.NextRunAt == nil {
		t.Error("Execute() NextRunAt = nil, want a scheduled time")
	}
	if got.LastRunAt == nil {
		t.Error("Execute() LastRunAt = nil, want a timestamp")
	}

	// Run record should exist with exit code 0.
	runs, err := db.GetRunsForJob(database, j.ID, 1)
	if err != nil {
		t.Fatalf("GetRunsForJob() unexpected error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("GetRunsForJob() = %d runs, want 1", len(runs))
	}
	if runs[0].ExitCode == nil || *runs[0].ExitCode != 0 {
		t.Errorf("run ExitCode = %v, want 0", runs[0].ExitCode)
	}
}

func TestExecute_Failure_NoRetries(t *testing.T) {
	database := openTestDB(t)
	j := addJob(t, database, sampleJob("fail", "exit 1"))

	if err := Execute(database, j, false); err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	got, err := db.GetJob(database, "fail")
	if err != nil {
		t.Fatalf("GetJob() unexpected error: %v", err)
	}
	if got.Status != job.StatusFailed {
		t.Errorf("Execute() status = %q, want %q", got.Status, job.StatusFailed)
	}

	runs, err := db.GetRunsForJob(database, j.ID, 1)
	if err != nil {
		t.Fatalf("GetRunsForJob() unexpected error: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("GetRunsForJob() = 0 runs, want 1")
	}
	if runs[0].ExitCode == nil || *runs[0].ExitCode == 0 {
		t.Errorf("run ExitCode = %v, want non-zero", runs[0].ExitCode)
	}
}

func TestExecute_Failure_WithRetries(t *testing.T) {
	database := openTestDB(t)
	j := sampleJob("retry", "exit 1")
	j.MaxRetries = 3
	j = addJob(t, database, j)

	if err := Execute(database, j, false); err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	got, err := db.GetJob(database, "retry")
	if err != nil {
		t.Fatalf("GetJob() unexpected error: %v", err)
	}
	// Should still be active (retries remaining), with a future next run.
	if got.Status != job.StatusActive {
		t.Errorf("Execute() status = %q, want %q", got.Status, job.StatusActive)
	}
	if got.NextRunAt == nil || !got.NextRunAt.After(time.Now()) {
		t.Error("Execute() NextRunAt should be in the future for a retry")
	}
}

func TestExecute_ConcurrencyGuard(t *testing.T) {
	database := openTestDB(t)
	j := addJob(t, database, sampleJob("hello", "echo hello"))

	// Simulate a run already in progress (no finished_at).
	openRun := &job.Run{JobID: j.ID, StartedAt: time.Now(), Attempt: 1}
	if _, err := db.AddRun(database, openRun); err != nil {
		t.Fatalf("AddRun() unexpected error: %v", err)
	}

	if err := Execute(database, j, false); err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	// Only the original open run should exist — Execute should have skipped.
	runs, err := db.GetRunsForJob(database, j.ID, 10)
	if err != nil {
		t.Fatalf("GetRunsForJob() unexpected error: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("Execute() created %d runs, want 1 (concurrency guard should skip)", len(runs))
	}
}

func TestExecute_Timeout(t *testing.T) {
	database := openTestDB(t)

	sleepCmd := "sleep 5"
	if runtime.GOOS == "windows" {
		sleepCmd = "ping -n 6 127.0.0.1 > NUL"
	}

	j := sampleJob("slow", sleepCmd)
	j.TimeoutSeconds = 1
	j = addJob(t, database, j)

	if err := Execute(database, j, false); err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	runs, err := db.GetRunsForJob(database, j.ID, 1)
	if err != nil {
		t.Fatalf("GetRunsForJob() unexpected error: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("GetRunsForJob() = 0 runs, want 1")
	}
	if runs[0].ExitCode == nil || *runs[0].ExitCode == 0 {
		t.Errorf("run ExitCode = %v, want non-zero (timeout kill)", runs[0].ExitCode)
	}
	if !strings.Contains(runs[0].Stderr, "killed") {
		t.Errorf("run Stderr = %q, want it to contain %q", runs[0].Stderr, "killed")
	}
}

func TestExecute_StdoutCaptured(t *testing.T) {
	database := openTestDB(t)
	j := addJob(t, database, sampleJob("hello", "echo hello"))

	if err := Execute(database, j, false); err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}

	runs, err := db.GetRunsForJob(database, j.ID, 1)
	if err != nil {
		t.Fatalf("GetRunsForJob() unexpected error: %v", err)
	}
	if len(runs) == 0 {
		t.Fatal("GetRunsForJob() = 0 runs, want 1")
	}
	if runs[0].Stdout == "" {
		t.Error("run Stdout is empty, want captured output")
	}
}

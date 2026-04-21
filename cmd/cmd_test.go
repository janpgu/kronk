package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/job"
	_ "modernc.org/sqlite"
)

// setupTestDB creates a temporary SQLite database, sets cfg.DBPath, and
// registers cleanup. Each test gets an isolated database file.
func setupTestDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	cfg.DBPath = filepath.Join(dir, "test.db")

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() unexpected error: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("db.Migrate() unexpected error: %v", err)
	}
	database.Close()
}

// addTestJob inserts a job directly into the test DB and returns it with its ID.
func addTestJob(t *testing.T, name, command, scheduleRaw, scheduleCron string) *job.Job {
	t.Helper()
	next := time.Now().Add(-time.Minute)
	j := &job.Job{
		Name:         name,
		Command:      command,
		ScheduleRaw:  scheduleRaw,
		ScheduleCron: scheduleCron,
		MaxRetries:   0,
		Status:       job.StatusActive,
		NextRunAt:    &next,
	}
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() unexpected error: %v", err)
	}
	defer database.Close()
	id, err := db.AddJob(database, j)
	if err != nil {
		t.Fatalf("db.AddJob(%q) unexpected error: %v", name, err)
	}
	j.ID = id
	return j
}

func TestRunAdd(t *testing.T) {
	setupTestDB(t)

	addName = "hello"
	addSchedule = "every minute"
	addRetries = 0
	if err := runAdd(nil, []string{"echo hello"}); err != nil {
		t.Fatalf("runAdd() unexpected error: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() unexpected error: %v", err)
	}
	defer database.Close()

	got, err := db.GetJob(database, "hello")
	if err != nil {
		t.Fatalf("db.GetJob() unexpected error: %v", err)
	}
	if got.Name != "hello" {
		t.Errorf("runAdd() Name = %q, want %q", got.Name, "hello")
	}
	if got.Command != "echo hello" {
		t.Errorf("runAdd() Command = %q, want %q", got.Command, "echo hello")
	}
	if got.ScheduleCron != "* * * * *" {
		t.Errorf("runAdd() ScheduleCron = %q, want %q", got.ScheduleCron, "* * * * *")
	}
	if got.NextRunAt == nil {
		t.Error("runAdd() NextRunAt = nil, want a scheduled time")
	}
}

func TestRunAdd_WithTimeout(t *testing.T) {
	setupTestDB(t)

	addName = "timed"
	addSchedule = "every minute"
	addRetries = 0
	addTimeout = 30
	if err := runAdd(nil, []string{"echo timed"}); err != nil {
		t.Fatalf("runAdd() unexpected error: %v", err)
	}
	addTimeout = 0 // reset

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() unexpected error: %v", err)
	}
	defer database.Close()

	got, err := db.GetJob(database, "timed")
	if err != nil {
		t.Fatalf("db.GetJob() unexpected error: %v", err)
	}
	if got.TimeoutSeconds != 30 {
		t.Errorf("runAdd() TimeoutSeconds = %d, want 30", got.TimeoutSeconds)
	}
}

func TestRunAdd_InvalidSchedule(t *testing.T) {
	setupTestDB(t)

	addName = "bad"
	addSchedule = "not a schedule"
	addRetries = 0
	if err := runAdd(nil, []string{"echo bad"}); err == nil {
		t.Error("runAdd() expected error for invalid schedule, got nil")
	}
}

func TestRunAdd_DuplicateName(t *testing.T) {
	setupTestDB(t)

	addName = "hello"
	addSchedule = "every minute"
	addRetries = 0
	if err := runAdd(nil, []string{"echo hello"}); err != nil {
		t.Fatalf("runAdd() first call unexpected error: %v", err)
	}
	if err := runAdd(nil, []string{"echo hello again"}); err == nil {
		t.Error("runAdd() expected error for duplicate name, got nil")
	}
}

func TestRunStatus_Empty(t *testing.T) {
	setupTestDB(t)

	if err := runStatus(nil, nil); err != nil {
		t.Fatalf("runStatus() on empty db unexpected error: %v", err)
	}
}

func TestRunStatus_WithJobs(t *testing.T) {
	setupTestDB(t)
	addTestJob(t, "hello", "echo hello", "every minute", "* * * * *")

	if err := runStatus(nil, nil); err != nil {
		t.Fatalf("runStatus() unexpected error: %v", err)
	}
}

func TestRunShow(t *testing.T) {
	setupTestDB(t)
	addTestJob(t, "hello", "echo hello", "every minute", "* * * * *")

	if err := runShow(nil, []string{"hello"}); err != nil {
		t.Fatalf("runShow() unexpected error: %v", err)
	}
}

func TestRunShow_NotFound(t *testing.T) {
	setupTestDB(t)

	if err := runShow(nil, []string{"nonexistent"}); err == nil {
		t.Error("runShow() expected error for missing job, got nil")
	}
}

func TestRunPauseAndResume(t *testing.T) {
	setupTestDB(t)
	addTestJob(t, "hello", "echo hello", "every minute", "* * * * *")

	// Pause the job.
	if err := runPause(nil, []string{"hello"}); err != nil {
		t.Fatalf("runPause() unexpected error: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() unexpected error: %v", err)
	}
	got, err := db.GetJob(database, "hello")
	database.Close()
	if err != nil {
		t.Fatalf("db.GetJob() unexpected error: %v", err)
	}
	if got.Status != job.StatusPaused {
		t.Errorf("runPause() Status = %q, want %q", got.Status, job.StatusPaused)
	}

	// Pause again — should be a no-op, not an error.
	if err := runPause(nil, []string{"hello"}); err != nil {
		t.Fatalf("runPause() second call unexpected error: %v", err)
	}

	// Resume the job.
	if err := runResume(nil, []string{"hello"}); err != nil {
		t.Fatalf("runResume() unexpected error: %v", err)
	}

	database, err = db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() unexpected error: %v", err)
	}
	got, err = db.GetJob(database, "hello")
	database.Close()
	if err != nil {
		t.Fatalf("db.GetJob() unexpected error: %v", err)
	}
	if got.Status != job.StatusActive {
		t.Errorf("runResume() Status = %q, want %q", got.Status, job.StatusActive)
	}
}

func TestRunRemove(t *testing.T) {
	setupTestDB(t)
	addTestJob(t, "hello", "echo hello", "every minute", "* * * * *")

	if err := runRemoveConfirmed("hello"); err != nil {
		t.Fatalf("runRemoveConfirmed() unexpected error: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() unexpected error: %v", err)
	}
	defer database.Close()

	if _, err := db.GetJob(database, "hello"); err == nil {
		t.Error("db.GetJob() expected error after removal, got nil")
	}
}

func TestRunRemove_NotFound(t *testing.T) {
	setupTestDB(t)

	if err := runRemoveConfirmed("nonexistent"); err == nil {
		t.Error("runRemoveConfirmed() expected error for missing job, got nil")
	}
}

func TestRunPrune(t *testing.T) {
	setupTestDB(t)
	j := addTestJob(t, "hello", "echo hello", "every minute", "* * * * *")

	// Insert an old run directly.
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() unexpected error: %v", err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if _, err := db.AddRun(database, &job.Run{JobID: j.ID, StartedAt: old, Attempt: 1}); err != nil {
		t.Fatalf("db.AddRun() unexpected error: %v", err)
	}
	database.Close()

	pruneDays = 1
	if err := runPrune(nil, nil); err != nil {
		t.Fatalf("runPrune() unexpected error: %v", err)
	}

	database, err = db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() unexpected error: %v", err)
	}
	defer database.Close()

	runs, err := db.GetRunsForJob(database, j.ID, 10)
	if err != nil {
		t.Fatalf("db.GetRunsForJob() unexpected error: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("runPrune() = %d runs remaining, want 0", len(runs))
	}
}

func TestRunTick(t *testing.T) {
	setupTestDB(t)
	addTestJob(t, "hello", "echo hello", "every minute", "* * * * *")

	if err := runTickLogic(false); err != nil {
		t.Fatalf("runTickLogic() unexpected error: %v", err)
	}

	// Job should have been executed — run record should exist.
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() unexpected error: %v", err)
	}
	defer database.Close()

	got, err := db.GetJob(database, "hello")
	if err != nil {
		t.Fatalf("db.GetJob() unexpected error: %v", err)
	}
	if got.LastRunAt == nil {
		t.Error("runTickLogic() LastRunAt = nil, want a timestamp")
	}
}

func TestRunTrigger(t *testing.T) {
	setupTestDB(t)
	addTestJob(t, "hello", "echo hello", "every minute", "* * * * *")

	if err := runTrigger(nil, []string{"hello"}); err != nil {
		t.Fatalf("runTrigger() unexpected error: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db.Open() unexpected error: %v", err)
	}
	defer database.Close()

	j, err := db.GetJob(database, "hello")
	if err != nil {
		t.Fatalf("db.GetJob() unexpected error: %v", err)
	}
	if j.LastRunAt == nil {
		t.Error("runTrigger() LastRunAt = nil, want a timestamp")
	}
}

func TestRunHistory_Empty(t *testing.T) {
	setupTestDB(t)

	historyJob = ""
	historyLimit = 20
	if err := runHistory(nil, nil); err != nil {
		t.Fatalf("runHistory() on empty db unexpected error: %v", err)
	}
}

func TestRunHistory_WithRuns(t *testing.T) {
	setupTestDB(t)
	addTestJob(t, "hello", "echo hello", "every minute", "* * * * *")

	// Trigger a run so there is history.
	if err := runTrigger(nil, []string{"hello"}); err != nil {
		t.Fatalf("runTrigger() unexpected error: %v", err)
	}

	historyJob = ""
	historyLimit = 20
	if err := runHistory(nil, nil); err != nil {
		t.Fatalf("runHistory() unexpected error: %v", err)
	}
}

func TestRunHistory_FilteredByJob(t *testing.T) {
	setupTestDB(t)
	addTestJob(t, "hello", "echo hello", "every minute", "* * * * *")

	if err := runTrigger(nil, []string{"hello"}); err != nil {
		t.Fatalf("runTrigger() unexpected error: %v", err)
	}

	historyJob = "hello"
	historyLimit = 20
	if err := runHistory(nil, nil); err != nil {
		t.Fatalf("runHistory(--job hello) unexpected error: %v", err)
	}
}

package db

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/janpgu/kronk/internal/job"
)

// openTestDB creates an in-memory SQLite database for testing.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open(':memory:') unexpected error: %v", err)
	}
	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate() unexpected error: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// sampleJob returns a job suitable for use in tests.
func sampleJob(name string) *job.Job {
	next := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)
	return &job.Job{
		Name:         name,
		Command:      "echo " + name,
		ScheduleRaw:  "every morning",
		ScheduleCron: "0 7 * * *",
		MaxRetries:   0,
		Status:       job.StatusActive,
		NextRunAt:    &next,
	}
}

func TestAddAndGetJob(t *testing.T) {
	database := openTestDB(t)

	j := sampleJob("hello")
	id, err := AddJob(database, j)
	if err != nil {
		t.Fatalf("AddJob() unexpected error: %v", err)
	}
	if id <= 0 {
		t.Errorf("AddJob() returned id %d, want > 0", id)
	}

	got, err := GetJob(database, "hello")
	if err != nil {
		t.Fatalf("GetJob() unexpected error: %v", err)
	}

	if got.Name != j.Name {
		t.Errorf("GetJob().Name = %q, want %q", got.Name, j.Name)
	}
	if got.Command != j.Command {
		t.Errorf("GetJob().Command = %q, want %q", got.Command, j.Command)
	}
	if got.ScheduleRaw != j.ScheduleRaw {
		t.Errorf("GetJob().ScheduleRaw = %q, want %q", got.ScheduleRaw, j.ScheduleRaw)
	}
	if got.Status != job.StatusActive {
		t.Errorf("GetJob().Status = %q, want %q", got.Status, job.StatusActive)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	database := openTestDB(t)

	_, err := GetJob(database, "nonexistent")
	if err == nil {
		t.Error("GetJob() expected error for missing job, got nil")
	}
}

func TestAddJob_DuplicateName(t *testing.T) {
	database := openTestDB(t)

	j := sampleJob("hello")
	if _, err := AddJob(database, j); err != nil {
		t.Fatalf("AddJob() first insert unexpected error: %v", err)
	}

	_, err := AddJob(database, j)
	if err == nil {
		t.Error("AddJob() expected error for duplicate name, got nil")
	}
}

func TestGetAllJobs(t *testing.T) {
	database := openTestDB(t)

	// Empty database returns empty slice without error.
	jobs, err := GetAllJobs(database)
	if err != nil {
		t.Fatalf("GetAllJobs() on empty db unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("GetAllJobs() on empty db = %d jobs, want 0", len(jobs))
	}

	// Add two jobs and verify both are returned ordered by name.
	names := []string{"zebra", "alpha"}
	for _, name := range names {
		if _, err := AddJob(database, sampleJob(name)); err != nil {
			t.Fatalf("AddJob(%q) unexpected error: %v", name, err)
		}
	}

	jobs, err = GetAllJobs(database)
	if err != nil {
		t.Fatalf("GetAllJobs() unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("GetAllJobs() = %d jobs, want 2", len(jobs))
	}
	if jobs[0].Name != "alpha" || jobs[1].Name != "zebra" {
		t.Errorf("GetAllJobs() order = [%q, %q], want [%q, %q]",
			jobs[0].Name, jobs[1].Name, "alpha", "zebra")
	}
}

func TestUpdateJob(t *testing.T) {
	database := openTestDB(t)

	if _, err := AddJob(database, sampleJob("hello")); err != nil {
		t.Fatalf("AddJob() unexpected error: %v", err)
	}

	j, err := GetJob(database, "hello")
	if err != nil {
		t.Fatalf("GetJob() unexpected error: %v", err)
	}

	j.Command = "echo updated"
	j.Status = job.StatusPaused
	if err := UpdateJob(database, j); err != nil {
		t.Fatalf("UpdateJob() unexpected error: %v", err)
	}

	got, err := GetJob(database, "hello")
	if err != nil {
		t.Fatalf("GetJob() after update unexpected error: %v", err)
	}
	if got.Command != "echo updated" {
		t.Errorf("UpdateJob() Command = %q, want %q", got.Command, "echo updated")
	}
	if got.Status != job.StatusPaused {
		t.Errorf("UpdateJob() Status = %q, want %q", got.Status, job.StatusPaused)
	}
}

func TestDeleteJob(t *testing.T) {
	database := openTestDB(t)

	if _, err := AddJob(database, sampleJob("hello")); err != nil {
		t.Fatalf("AddJob() unexpected error: %v", err)
	}

	if err := DeleteJob(database, "hello"); err != nil {
		t.Fatalf("DeleteJob() unexpected error: %v", err)
	}

	_, err := GetJob(database, "hello")
	if err == nil {
		t.Error("GetJob() expected error after deletion, got nil")
	}
}

func TestDeleteJob_NotFound(t *testing.T) {
	database := openTestDB(t)

	err := DeleteJob(database, "nonexistent")
	if err == nil {
		t.Error("DeleteJob() expected error for missing job, got nil")
	}
}

func TestGetDueJobs(t *testing.T) {
	database := openTestDB(t)

	past := time.Now().Add(-time.Minute)
	future := time.Now().Add(time.Hour)

	due := &job.Job{
		Name: "due", Command: "echo due",
		ScheduleRaw: "every minute", ScheduleCron: "* * * * *",
		Status: job.StatusActive, NextRunAt: &past,
	}
	notDue := &job.Job{
		Name: "not-due", Command: "echo not-due",
		ScheduleRaw: "every hour", ScheduleCron: "0 * * * *",
		Status: job.StatusActive, NextRunAt: &future,
	}
	paused := &job.Job{
		Name: "paused", Command: "echo paused",
		ScheduleRaw: "every minute", ScheduleCron: "* * * * *",
		Status: job.StatusPaused, NextRunAt: &past,
	}

	for _, j := range []*job.Job{due, notDue, paused} {
		id, err := AddJob(database, j)
		if err != nil {
			t.Fatalf("AddJob(%q) unexpected error: %v", j.Name, err)
		}
		j.ID = id
		// AddJob always inserts as active; update status separately if needed.
		if j.Status != job.StatusActive {
			if err := UpdateJob(database, j); err != nil {
				t.Fatalf("UpdateJob(%q) unexpected error: %v", j.Name, err)
			}
		}
	}

	jobs, err := GetDueJobs(database)
	if err != nil {
		t.Fatalf("GetDueJobs() unexpected error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("GetDueJobs() = %d jobs, want 1", len(jobs))
	}
	if jobs[0].Name != "due" {
		t.Errorf("GetDueJobs()[0].Name = %q, want %q", jobs[0].Name, "due")
	}
}

func TestAddAndUpdateRun(t *testing.T) {
	database := openTestDB(t)

	id, err := AddJob(database, sampleJob("hello"))
	if err != nil {
		t.Fatalf("AddJob() unexpected error: %v", err)
	}

	r := &job.Run{
		JobID:     id,
		StartedAt: time.Now(),
		Attempt:   1,
	}
	runID, err := AddRun(database, r)
	if err != nil {
		t.Fatalf("AddRun() unexpected error: %v", err)
	}
	if runID <= 0 {
		t.Errorf("AddRun() returned id %d, want > 0", runID)
	}

	// Verify the run appears as a running instance.
	running, err := HasRunningInstance(database, id)
	if err != nil {
		t.Fatalf("HasRunningInstance() unexpected error: %v", err)
	}
	if !running {
		t.Error("HasRunningInstance() = false, want true while run is open")
	}

	// Complete the run.
	finished := time.Now()
	exitCode := 0
	r.ID = runID
	r.FinishedAt = &finished
	r.ExitCode = &exitCode
	r.Stdout = "hello"
	r.Stderr = ""
	if err := UpdateRun(database, r); err != nil {
		t.Fatalf("UpdateRun() unexpected error: %v", err)
	}

	// Verify no longer running.
	running, err = HasRunningInstance(database, id)
	if err != nil {
		t.Fatalf("HasRunningInstance() unexpected error: %v", err)
	}
	if running {
		t.Error("HasRunningInstance() = true, want false after run completed")
	}
}

func TestGetRunsForJob(t *testing.T) {
	database := openTestDB(t)

	jobID, err := AddJob(database, sampleJob("hello"))
	if err != nil {
		t.Fatalf("AddJob() unexpected error: %v", err)
	}

	// Add three runs.
	for i := 1; i <= 3; i++ {
		r := &job.Run{JobID: jobID, StartedAt: time.Now(), Attempt: i}
		if _, err := AddRun(database, r); err != nil {
			t.Fatalf("AddRun() attempt %d unexpected error: %v", i, err)
		}
	}

	tests := []struct {
		limit int
		want  int
	}{
		{1, 1},
		{2, 2},
		{10, 3},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("limit=%d", tt.limit), func(t *testing.T) {
			runs, err := GetRunsForJob(database, jobID, tt.limit)
			if err != nil {
				t.Fatalf("GetRunsForJob(limit=%d) unexpected error: %v", tt.limit, err)
			}
			if len(runs) != tt.want {
				t.Errorf("GetRunsForJob(limit=%d) = %d runs, want %d", tt.limit, len(runs), tt.want)
			}
		})
	}
}

func TestPruneRuns(t *testing.T) {
	database := openTestDB(t)

	jobID, err := AddJob(database, sampleJob("hello"))
	if err != nil {
		t.Fatalf("AddJob() unexpected error: %v", err)
	}

	old := time.Now().Add(-48 * time.Hour)
	recent := time.Now().Add(-time.Minute)

	// Add one old run and one recent run.
	if _, err := AddRun(database, &job.Run{JobID: jobID, StartedAt: old, Attempt: 1}); err != nil {
		t.Fatalf("AddRun(old) unexpected error: %v", err)
	}
	if _, err := AddRun(database, &job.Run{JobID: jobID, StartedAt: recent, Attempt: 2}); err != nil {
		t.Fatalf("AddRun(recent) unexpected error: %v", err)
	}

	// Prune runs older than 1 day — should remove the old run only.
	before := time.Now().Add(-24 * time.Hour)
	n, err := PruneRuns(database, before)
	if err != nil {
		t.Fatalf("PruneRuns() unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("PruneRuns() = %d deleted, want 1", n)
	}

	runs, err := GetRunsForJob(database, jobID, 10)
	if err != nil {
		t.Fatalf("GetRunsForJob() unexpected error: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("GetRunsForJob() after prune = %d runs, want 1", len(runs))
	}
}

func TestDeleteJob_CascadesRuns(t *testing.T) {
	database := openTestDB(t)

	jobID, err := AddJob(database, sampleJob("hello"))
	if err != nil {
		t.Fatalf("AddJob() unexpected error: %v", err)
	}

	// Add a run for the job.
	if _, err := AddRun(database, &job.Run{JobID: jobID, StartedAt: time.Now(), Attempt: 1}); err != nil {
		t.Fatalf("AddRun() unexpected error: %v", err)
	}

	if err := DeleteJob(database, "hello"); err != nil {
		t.Fatalf("DeleteJob() unexpected error: %v", err)
	}

	// Runs for the deleted job should be gone.
	runs, err := GetRunsForJob(database, jobID, 10)
	if err != nil {
		t.Fatalf("GetRunsForJob() unexpected error: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("GetRunsForJob() after delete = %d runs, want 0", len(runs))
	}
}

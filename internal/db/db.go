// Package db handles all SQLite interactions for kronk.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/janpgu/kronk/internal/job"
	_ "modernc.org/sqlite" // registers the "sqlite" driver, used via database/sql
)

// Open opens (or creates) the SQLite database at the given path.
// The directory is created automatically if it does not exist.
func Open(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("could not create database directory %q: %w", dir, err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}

	// Verify the connection is actually usable.
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("could not connect to database: %w", err)
	}

	return db, nil
}

// Migrate ensures all required tables exist. Safe to call on every startup —
// CREATE TABLE IF NOT EXISTS is idempotent and never destroys existing data.
func Migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id            INTEGER PRIMARY KEY,
			name          TEXT UNIQUE NOT NULL,
			command       TEXT NOT NULL,
			schedule_raw  TEXT NOT NULL,
			schedule_cron TEXT NOT NULL,
			max_retries   INTEGER DEFAULT 0,
			status        TEXT DEFAULT 'active',
			created_at    DATETIME,
			updated_at    DATETIME,
			last_run_at   DATETIME,
			next_run_at   DATETIME
		);

		CREATE TABLE IF NOT EXISTS runs (
			id          INTEGER PRIMARY KEY,
			job_id      INTEGER REFERENCES jobs(id),
			started_at  DATETIME,
			finished_at DATETIME,
			exit_code   INTEGER,
			stdout      TEXT,
			stderr      TEXT,
			attempt     INTEGER DEFAULT 1
		);
	`)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	return nil
}

// AddJob inserts a new job and returns the assigned ID.
func AddJob(db *sql.DB, j *job.Job) (int64, error) {
	now := time.Now()
	result, err := db.Exec(`
		INSERT INTO jobs (name, command, schedule_raw, schedule_cron, max_retries, status, created_at, updated_at, next_run_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.Name, j.Command, j.ScheduleRaw, j.ScheduleCron, j.MaxRetries, job.StatusActive, now, now, j.NextRunAt,
	)
	if err != nil {
		return 0, fmt.Errorf("could not add job %q: %w", j.Name, err)
	}
	return result.LastInsertId()
}

// GetJob retrieves a single job by name. Returns an error if not found.
func GetJob(db *sql.DB, name string) (*job.Job, error) {
	row := db.QueryRow(`SELECT * FROM jobs WHERE name = ?`, name)
	j, err := scanJob(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job %q not found", name)
	}
	if err != nil {
		return nil, fmt.Errorf("could not get job %q: %w", name, err)
	}
	return j, nil
}

// GetAllJobs returns all jobs ordered by name.
func GetAllJobs(db *sql.DB) ([]*job.Job, error) {
	rows, err := db.Query(`SELECT * FROM jobs ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("could not list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*job.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("could not read job row: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// GetDueJobs returns all active jobs whose next_run_at is in the past.
func GetDueJobs(db *sql.DB) ([]*job.Job, error) {
	rows, err := db.Query(`
		SELECT * FROM jobs
		WHERE status = 'active'
		AND next_run_at IS NOT NULL
		AND next_run_at <= ?`,
		time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not query due jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*job.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("could not read job row: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// UpdateJob saves changes to an existing job record.
func UpdateJob(db *sql.DB, j *job.Job) error {
	_, err := db.Exec(`
		UPDATE jobs SET
			command       = ?,
			schedule_raw  = ?,
			schedule_cron = ?,
			max_retries   = ?,
			status        = ?,
			updated_at    = ?,
			last_run_at   = ?,
			next_run_at   = ?
		WHERE id = ?`,
		j.Command, j.ScheduleRaw, j.ScheduleCron, j.MaxRetries,
		j.Status, time.Now(), j.LastRunAt, j.NextRunAt, j.ID,
	)
	if err != nil {
		return fmt.Errorf("could not update job %q: %w", j.Name, err)
	}
	return nil
}

// DeleteJob removes a job and all its run history.
func DeleteJob(db *sql.DB, name string) error {
	j, err := GetJob(db, name)
	if err != nil {
		return err
	}

	if _, err := db.Exec(`DELETE FROM runs WHERE job_id = ?`, j.ID); err != nil {
		return fmt.Errorf("could not delete runs for job %q: %w", name, err)
	}
	if _, err := db.Exec(`DELETE FROM jobs WHERE id = ?`, j.ID); err != nil {
		return fmt.Errorf("could not delete job %q: %w", name, err)
	}
	return nil
}

// AddRun inserts a new run record and returns its ID.
func AddRun(db *sql.DB, r *job.Run) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO runs (job_id, started_at, attempt)
		VALUES (?, ?, ?)`,
		r.JobID, r.StartedAt, r.Attempt,
	)
	if err != nil {
		return 0, fmt.Errorf("could not record run start: %w", err)
	}
	return result.LastInsertId()
}

// UpdateRun saves the result of a completed run.
func UpdateRun(db *sql.DB, r *job.Run) error {
	_, err := db.Exec(`
		UPDATE runs SET
			finished_at = ?,
			exit_code   = ?,
			stdout      = ?,
			stderr      = ?
		WHERE id = ?`,
		r.FinishedAt, r.ExitCode, r.Stdout, r.Stderr, r.ID,
	)
	if err != nil {
		return fmt.Errorf("could not update run record: %w", err)
	}
	return nil
}

// GetRunsForJob returns recent runs for a job, newest first.
func GetRunsForJob(db *sql.DB, jobID int64, limit int) ([]*job.Run, error) {
	rows, err := db.Query(`
		SELECT id, job_id, started_at, finished_at, exit_code, stdout, stderr, attempt
		FROM runs
		WHERE job_id = ?
		ORDER BY started_at DESC
		LIMIT ?`,
		jobID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query runs: %w", err)
	}
	defer rows.Close()

	var runs []*job.Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("could not read run row: %w", err)
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// GetAllRuns returns recent runs across all jobs, newest first.
func GetAllRuns(db *sql.DB, limit int) ([]*job.Run, error) {
	rows, err := db.Query(`
		SELECT id, job_id, started_at, finished_at, exit_code, stdout, stderr, attempt
		FROM runs
		ORDER BY started_at DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query runs: %w", err)
	}
	defer rows.Close()

	var runs []*job.Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("could not read run row: %w", err)
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// RunWithName extends Run with the human-readable job name for display purposes.
type RunWithName struct {
	*job.Run
	JobName string
}

// AttachJobName wraps a slice of runs with a single job name.
func AttachJobName(runs []*job.Run, name string) []*RunWithName {
	out := make([]*RunWithName, len(runs))
	for i, r := range runs {
		out[i] = &RunWithName{Run: r, JobName: name}
	}
	return out
}

// GetAllRunsWithNames returns recent runs across all jobs with their job names, newest first.
func GetAllRunsWithNames(db *sql.DB, limit int) ([]*RunWithName, error) {
	rows, err := db.Query(`
		SELECT r.id, r.job_id, r.started_at, r.finished_at, r.exit_code, r.stdout, r.stderr, r.attempt,
		       j.name
		FROM runs r
		JOIN jobs j ON j.id = r.job_id
		ORDER BY r.started_at DESC
		LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query runs: %w", err)
	}
	defer rows.Close()

	var runs []*RunWithName
	for rows.Next() {
		var r job.Run
		var name string
		err := rows.Scan(
			&r.ID, &r.JobID,
			&r.StartedAt, &r.FinishedAt,
			&r.ExitCode, &r.Stdout, &r.Stderr,
			&r.Attempt, &name,
		)
		if err != nil {
			return nil, fmt.Errorf("could not read run row: %w", err)
		}
		runs = append(runs, &RunWithName{Run: &r, JobName: name})
	}
	return runs, rows.Err()
}

// HasRunningInstance reports whether a job has an unfinished run in the database.
// Used as a concurrency guard before starting a new execution.
func HasRunningInstance(db *sql.DB, jobID int64) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM runs
		WHERE job_id = ? AND finished_at IS NULL`,
		jobID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("could not check for running instance: %w", err)
	}
	return count > 0, nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows, letting scanJob work for both.
type scanner interface {
	Scan(dest ...any) error
}

// scanJob reads one row from the jobs table into a Job struct.
func scanJob(s scanner) (*job.Job, error) {
	var j job.Job
	var status string
	err := s.Scan(
		&j.ID, &j.Name, &j.Command,
		&j.ScheduleRaw, &j.ScheduleCron,
		&j.MaxRetries, &status,
		&j.CreatedAt, &j.UpdatedAt,
		&j.LastRunAt, &j.NextRunAt,
	)
	if err != nil {
		return nil, err
	}
	j.Status = job.Status(status)
	return &j, nil
}

// scanRun reads one row from the runs table into a Run struct.
// stdout and stderr are scanned via nullable intermediates because they are
// NULL for runs that have not yet completed.
func scanRun(s scanner) (*job.Run, error) {
	var r job.Run
	var stdout, stderr *string
	err := s.Scan(
		&r.ID, &r.JobID,
		&r.StartedAt, &r.FinishedAt,
		&r.ExitCode, &stdout, &stderr,
		&r.Attempt,
	)
	if err != nil {
		return nil, err
	}
	if stdout != nil {
		r.Stdout = *stdout
	}
	if stderr != nil {
		r.Stderr = *stderr
	}
	return &r, nil
}

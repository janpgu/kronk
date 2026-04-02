package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/janpgu/kronk/internal/db"
	"github.com/janpgu/kronk/internal/job"
	"github.com/janpgu/kronk/internal/scheduler"
	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit all jobs in $EDITOR",
	RunE:  runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(cmd *cobra.Command, args []string) error {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		return err
	}

	jobs, err := db.GetAllJobs(database)
	if err != nil {
		return err
	}

	// Write jobs to a temp file in a simple editable format.
	tmpFile, err := os.CreateTemp("", "kronk-edit-*.txt")
	if err != nil {
		return fmt.Errorf("could not create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	fmt.Fprintln(tmpFile, "# kronk job editor")
	fmt.Fprintln(tmpFile, "# Format: name | schedule | retries | command")
	fmt.Fprintln(tmpFile, "# - Edit existing lines to update jobs")
	fmt.Fprintln(tmpFile, "# - Add new lines to create jobs")
	fmt.Fprintln(tmpFile, "# - Delete lines to remove jobs (you will be prompted)")
	fmt.Fprintln(tmpFile, "# - Lines starting with # are ignored")
	fmt.Fprintln(tmpFile)
	for _, j := range jobs {
		fmt.Fprintf(tmpFile, "%s | %s | %d | %s\n", j.Name, j.ScheduleRaw, j.MaxRetries, j.Command)
	}
	tmpFile.Close()

	// Record the file's modification time before opening the editor.
	statBefore, err := os.Stat(tmpPath)
	if err != nil {
		return fmt.Errorf("could not stat temp file: %w", err)
	}

	// Open the editor.
	if err := openEditor(tmpPath); err != nil {
		return err
	}

	// If the file was not modified, do nothing.
	statAfter, err := os.Stat(tmpPath)
	if err != nil {
		return fmt.Errorf("could not stat temp file after edit: %w", err)
	}
	if statAfter.ModTime().Equal(statBefore.ModTime()) {
		fmt.Println(ui.MutedStyle.Render("No changes."))
		return nil
	}

	// Parse the edited file.
	edited, err := parseEditFile(tmpPath)
	if err != nil {
		return err
	}

	// Validate all schedules before applying anything, and cache results.
	cronExprs := make(map[string]string, len(edited))
	for _, e := range edited {
		cronExpr, err := scheduler.Parse(e.scheduleRaw)
		if err != nil {
			return fmt.Errorf("invalid schedule for job %q: %w", e.name, err)
		}
		cronExprs[e.name] = cronExpr
	}

	// Build lookup maps for diffing.
	existing := make(map[string]*job.Job, len(jobs))
	for _, j := range jobs {
		existing[j.Name] = j
	}
	editedMap := make(map[string]editedJob, len(edited))
	for _, e := range edited {
		editedMap[e.name] = e
	}

	// Compute diff.
	var toAdd, toUpdate []editedJob
	var toRemove []string

	for _, e := range edited {
		if _, exists := existing[e.name]; !exists {
			toAdd = append(toAdd, e)
		} else {
			toUpdate = append(toUpdate, e)
		}
	}
	for _, j := range jobs {
		if _, exists := editedMap[j.Name]; !exists {
			toRemove = append(toRemove, j.Name)
		}
	}

	if len(toAdd)+len(toUpdate)+len(toRemove) == 0 {
		fmt.Println(ui.MutedStyle.Render("No changes."))
		return nil
	}

	// Print summary and prompt for removals.
	if len(toAdd) > 0 {
		for _, e := range toAdd {
			fmt.Printf("  %s add %s\n", ui.SuccessStyle.Render("+"), e.name)
		}
	}
	if len(toUpdate) > 0 {
		for _, e := range toUpdate {
			fmt.Printf("  %s update %s\n", ui.WarnStyle.Render("~"), e.name)
		}
	}
	if len(toRemove) > 0 {
		for _, name := range toRemove {
			fmt.Printf("  %s remove %s\n", ui.ErrorStyle.Render("-"), name)
		}
		fmt.Printf("\nRemove %d job(s) and their history? [y/N] ", len(toRemove))
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Apply changes.
	for _, e := range toAdd {
		cronExpr := cronExprs[e.name]
		nextRun, err := scheduler.NextRun(cronExpr, time.Now())
		if err != nil {
			return err
		}
		j := &job.Job{
			Name:         e.name,
			Command:      e.command,
			ScheduleRaw:  e.scheduleRaw,
			ScheduleCron: cronExpr,
			MaxRetries:   e.retries,
			NextRunAt:    &nextRun,
		}
		if _, err := db.AddJob(database, j); err != nil {
			return err
		}
	}

	for _, e := range toUpdate {
		j := existing[e.name]
		cronExpr := cronExprs[e.name]
		nextRun, err := scheduler.NextRun(cronExpr, time.Now())
		if err != nil {
			return err
		}
		j.Command = e.command
		j.ScheduleRaw = e.scheduleRaw
		j.ScheduleCron = cronExpr
		j.MaxRetries = e.retries
		j.NextRunAt = &nextRun
		if err := db.UpdateJob(database, j); err != nil {
			return err
		}
	}

	for _, name := range toRemove {
		if err := db.DeleteJob(database, name); err != nil {
			return err
		}
	}

	fmt.Printf("\n%d added, %d updated, %d removed\n", len(toAdd), len(toUpdate), len(toRemove))
	return nil
}

// editedJob holds one parsed line from the edit file.
type editedJob struct {
	name        string
	scheduleRaw string
	retries     int
	command     string
}

// parseEditFile reads the temp file and returns the parsed jobs.
func parseEditFile(path string) ([]editedJob, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not read edit file: %w", err)
	}
	defer f.Close()

	var jobs []editedJob
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			return nil, fmt.Errorf("line %d: expected format 'name | schedule | retries | command', got: %q", lineNum, line)
		}
		name := strings.TrimSpace(parts[0])
		scheduleRaw := strings.TrimSpace(parts[1])
		retriesStr := strings.TrimSpace(parts[2])
		command := strings.TrimSpace(parts[3])

		if name == "" || scheduleRaw == "" || command == "" {
			return nil, fmt.Errorf("line %d: name, schedule, and command must not be empty", lineNum)
		}

		retries, err := strconv.Atoi(retriesStr)
		if err != nil {
			return nil, fmt.Errorf("line %d: retries must be a number, got %q", lineNum, retriesStr)
		}

		jobs = append(jobs, editedJob{name, scheduleRaw, retries, command})
	}
	return jobs, scanner.Err()
}

// openEditor opens the given file in the user's preferred editor.
func openEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "nano"
		}
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %q exited with error: %w", editor, err)
	}
	return nil
}

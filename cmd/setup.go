package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/janpgu/kronk/internal/ui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Register kronk with the system scheduler (cron or Task Scheduler)",
	Long: `Sets up the system scheduler so that kronk runs automatically every minute.

On Linux and macOS, adds a crontab entry.
On Windows, writes a silent VBScript launcher and registers a Task Scheduler task.

Safe to run multiple times — existing entries are updated, not duplicated.`,
	Args: cobra.NoArgs,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine binary path: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		return setupWindows(binary)
	default:
		return setupUnix(binary)
	}
}

// setupUnix adds a crontab entry for `kronk tick` if one is not already present.
func setupUnix(binary string) error {
	cronLine := fmt.Sprintf("* * * * * %s tick", binary)

	// Read existing crontab.
	out, err := exec.Command("crontab", "-l").Output()
	existing := ""
	if err == nil {
		existing = string(out)
	}
	// A non-zero exit from `crontab -l` just means no crontab exists yet — not an error.

	if strings.Contains(existing, binary+" tick") {
		ui.PrintSuccess("Crontab entry already present — nothing to do.")
		return nil
	}

	// Append the new entry and install.
	updated := strings.TrimRight(existing, "\n") + "\n" + cronLine + "\n"
	pipe := exec.Command("crontab", "-")
	pipe.Stdin = strings.NewReader(updated)
	pipe.Stdout = os.Stdout
	pipe.Stderr = os.Stderr
	if err := pipe.Run(); err != nil {
		return fmt.Errorf("could not update crontab: %w", err)
	}

	ui.PrintSuccess(fmt.Sprintf("Crontab entry added: %s", cronLine))
	return nil
}

// setupWindows writes a silent VBScript launcher and registers a Task Scheduler task.
func setupWindows(binary string) error {
	installDir := filepath.Dir(binary)
	vbsPath := filepath.Join(installDir, "kronk-tick.vbs")

	// Write the VBScript wrapper that launches kronk tick with no console window.
	vbsContent := fmt.Sprintf(`CreateObject("WScript.Shell").Run "%s tick", 0, False`, binary)
	if err := os.WriteFile(vbsPath, []byte(vbsContent), 0644); err != nil {
		return fmt.Errorf("could not write launcher script: %w", err)
	}
	ui.PrintSuccess(fmt.Sprintf("Silent launcher written: %s", vbsPath))

	// Build the Task Scheduler XML definition.
	// Note: no encoding declaration — schtasks accepts UTF-8 without it.
	taskXML := fmt.Sprintf(`<?xml version="1.0"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>kronk job scheduler tick</Description>
  </RegistrationInfo>
  <Triggers>
    <TimeTrigger>
      <Repetition>
        <Interval>PT1M</Interval>
        <StopAtDurationEnd>false</StopAtDurationEnd>
      </Repetition>
      <StartBoundary>2000-01-01T00:00:00</StartBoundary>
      <Enabled>true</Enabled>
    </TimeTrigger>
  </Triggers>
  <Settings>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <ExecutionTimeLimit>PT5M</ExecutionTimeLimit>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <Enabled>true</Enabled>
  </Settings>
  <Actions>
    <Exec>
      <Command>wscript.exe</Command>
      <Arguments>//B "%s"</Arguments>
    </Exec>
  </Actions>
</Task>`, vbsPath)

	// Write XML to a temp file — schtasks requires a file path.
	xmlPath := filepath.Join(os.TempDir(), "kronk-task.xml")
	if err := os.WriteFile(xmlPath, []byte(taskXML), 0644); err != nil {
		return fmt.Errorf("could not write task XML: %w", err)
	}
	defer os.Remove(xmlPath)

	// Remove existing task silently before re-creating.
	exec.Command("schtasks", "/delete", "/tn", "kronk", "/f").Run() //nolint:errcheck

	// Register the task.
	out, err := exec.Command("schtasks", "/create", "/tn", "kronk", "/xml", xmlPath, "/f").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not register Task Scheduler task: %s", strings.TrimSpace(string(out)))
	}

	ui.PrintSuccess("Task Scheduler task 'kronk' registered (runs every minute, including on battery)")
	return nil
}

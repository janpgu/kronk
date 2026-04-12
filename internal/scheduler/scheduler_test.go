package scheduler

import (
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		// Fixed natural language patterns.
		{"every minute", "* * * * *", false},
		{"every hour", "0 * * * *", false},
		{"every night", "0 2 * * *", false},
		{"every morning", "0 7 * * *", false},
		{"every day", "0 9 * * *", false},
		{"every weekday", "0 9 * * 1-5", false},
		{"every weekend", "0 10 * * 6,0", false},
		{"twice a day", "0 9,21 * * *", false},

		// Case insensitivity.
		{"Every Night", "0 2 * * *", false},
		{"EVERY MINUTE", "* * * * *", false},

		// Dynamic: every N minutes.
		{"every 5 minutes", "*/5 * * * *", false},
		{"every 30 minutes", "*/30 * * * *", false},
		{"every 1 minute", "*/1 * * * *", false},

		// Dynamic: every N hours.
		{"every 2 hours", "0 */2 * * *", false},
		{"every 6 hours", "0 */6 * * *", false},

		// Every day at a specific time.
		{"every day at 9am", "0 9 * * *", false},
		{"every day at 3pm", "0 15 * * *", false},
		{"every day at 12pm", "0 12 * * *", false},
		{"every day at 12am", "0 0 * * *", false},
		{"every day at 9:30am", "30 9 * * *", false},
		{"every day at 3:30pm", "30 15 * * *", false},

		// Weekdays.
		{"every monday", "0 9 * * 1", false},
		{"every tuesday", "0 9 * * 2", false},
		{"every wednesday", "0 9 * * 3", false},
		{"every thursday", "0 9 * * 4", false},
		{"every friday", "0 9 * * 5", false},
		{"every saturday", "0 9 * * 6", false},
		{"every sunday", "0 9 * * 0", false},

		// Weekday at a specific time.
		{"every monday at 9am", "0 9 * * 1", false},
		{"every friday at 3pm", "0 15 * * 5", false},
		{"every wednesday at 10:30am", "30 10 * * 3", false},

		// Raw cron pass-through.
		{"0 2 * * *", "0 2 * * *", false},
		{"*/5 * * * *", "*/5 * * * *", false},
		{"30 9 * * 1-5", "30 9 * * 1-5", false},

		// Whitespace trimming.
		{"  every night  ", "0 2 * * *", false},

		// Errors.
		{"every 0 minutes", "", true},
		{"every 60 minutes", "", true},
		{"every 0 hours", "", true},
		{"every 24 hours", "", true},
		{"every fortnight", "", true},
		{"", "", true},
		{"not a schedule", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse(%q) expected error, got %q", tt.input, got)
				}
				return
			}

			if err != nil {
				t.Errorf("Parse(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got != tt.want {
				t.Errorf("Parse(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParse_DidYouMean(t *testing.T) {
	tests := []struct {
		input      string
		suggestion string
	}{
		{"every nigt", "every night"},
		{"every minite", "every minute"},
		{"every mourning", "every morning"},
		{"every weakday", "every weekday"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err == nil {
				t.Fatalf("Parse(%q) expected error, got nil", tt.input)
			}
			if !strings.Contains(err.Error(), tt.suggestion) {
				t.Errorf("Parse(%q) error = %q, want suggestion %q", tt.input, err.Error(), tt.suggestion)
			}
		})
	}
}

func TestNextRun(t *testing.T) {
	// Fixed reference time: Monday 2026-03-30 09:00:00 UTC.
	from := time.Date(2026, 3, 30, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		cron string
		want time.Time
	}{
		// Next minute.
		{"* * * * *", time.Date(2026, 3, 30, 9, 1, 0, 0, time.UTC)},
		// Next hour.
		{"0 * * * *", time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)},
		// Tonight at 2am — crosses midnight.
		{"0 2 * * *", time.Date(2026, 3, 31, 2, 0, 0, 0, time.UTC)},
		// Next Tuesday (from Monday).
		{"0 9 * * 2", time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.cron, func(t *testing.T) {
			got, err := NextRun(tt.cron, from)
			if err != nil {
				t.Fatalf("NextRun(%q) unexpected error: %v", tt.cron, err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("NextRun(%q) = %v, want %v", tt.cron, got, tt.want)
			}
		})
	}
}

// Package scheduler handles schedule parsing and next-run calculation.
package scheduler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// example holds a representative input and its expected cron output, used in
// error messages and did-you-mean suggestions.
type example struct {
	input string
	cron  string
}

// rule maps a natural language pattern to a cron expression.
// If cronExpr is empty, the handler function is called instead.
// examples holds representative inputs shown in error messages and used for
// did-you-mean matching — keeping all schedule knowledge in one place.
type rule struct {
	pattern  *regexp.Regexp
	cronExpr string
	handler  func(matches []string) (string, error)
	examples []example
}

// rules is evaluated in order — more specific patterns must come before general ones.
var rules = []rule{
	// "every minute"
	{
		pattern:  regexp.MustCompile(`(?i)^every minute$`),
		cronExpr: "* * * * *",
		examples: []example{{"every minute", "* * * * *"}},
	},
	// "every 5 minutes", "every 30 minutes"
	{
		pattern: regexp.MustCompile(`(?i)^every (\d+) minutes?$`),
		handler: func(m []string) (string, error) {
			n, _ := strconv.Atoi(m[1])
			if n < 1 || n > 59 {
				return "", fmt.Errorf("minutes must be between 1 and 59, got %d", n)
			}
			return fmt.Sprintf("*/%d * * * *", n), nil
		},
		examples: []example{{"every 5 minutes", "*/5 * * * *"}},
	},
	// "every hour"
	{
		pattern:  regexp.MustCompile(`(?i)^every hour$`),
		cronExpr: "0 * * * *",
		examples: []example{{"every hour", "0 * * * *"}},
	},
	// "every 2 hours", "every 6 hours"
	{
		pattern: regexp.MustCompile(`(?i)^every (\d+) hours?$`),
		handler: func(m []string) (string, error) {
			n, _ := strconv.Atoi(m[1])
			if n < 1 || n > 23 {
				return "", fmt.Errorf("hours must be between 1 and 23, got %d", n)
			}
			return fmt.Sprintf("0 */%d * * *", n), nil
		},
		examples: []example{{"every 6 hours", "0 */6 * * *"}},
	},
	// "every day at 9am", "every day at 3pm", "every day at 9:30am"
	{
		pattern: regexp.MustCompile(`(?i)^every day at (\d{1,2})(?::(\d{2}))?([ap]m)$`),
		handler: func(m []string) (string, error) {
			hour, min, err := parseHourMin(m[1], m[2], m[3])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d %d * * *", min, hour), nil
		},
		examples: []example{
			{"every day at 9am", "0 9 * * *"},
			{"every day at 3:30pm", "30 15 * * *"},
		},
	},
	// "every night" → 2am
	{
		pattern:  regexp.MustCompile(`(?i)^every night$`),
		cronExpr: "0 2 * * *",
		examples: []example{{"every night", "0 2 * * *"}},
	},
	// "every morning" → 7am
	{
		pattern:  regexp.MustCompile(`(?i)^every morning$`),
		cronExpr: "0 7 * * *",
		examples: []example{{"every morning", "0 7 * * *"}},
	},
	// "every monday at 9am" etc — must come before "every monday"
	{
		pattern:  regexp.MustCompile(`(?i)^every (monday|tuesday|wednesday|thursday|friday|saturday|sunday) at (\d{1,2})(?::(\d{2}))?([ap]m)$`),
		handler:  parseDayAtTime,
		examples: []example{{"every monday at 9am", "0 9 * * 1"}},
	},
	// "every monday", "every friday"
	{
		pattern: regexp.MustCompile(`(?i)^every (monday|tuesday|wednesday|thursday|friday|saturday|sunday)$`),
		handler: func(m []string) (string, error) {
			return fmt.Sprintf("0 9 * * %d", dayNumber(m[1])), nil
		},
		examples: []example{{"every monday", "0 9 * * 1"}},
	},
	// "every weekday" → Mon-Fri at 9am
	{
		pattern:  regexp.MustCompile(`(?i)^every weekday$`),
		cronExpr: "0 9 * * 1-5",
		examples: []example{{"every weekday", "0 9 * * 1-5"}},
	},
	// "every weekend" → Sat+Sun at 10am
	{
		pattern:  regexp.MustCompile(`(?i)^every weekend$`),
		cronExpr: "0 10 * * 6,0",
		examples: []example{{"every weekend", "0 10 * * 6,0"}},
	},
	// "twice a day" → 9am and 9pm
	{
		pattern:  regexp.MustCompile(`(?i)^twice a day$`),
		cronExpr: "0 9,21 * * *",
		examples: []example{{"twice a day", "0 9,21 * * *"}},
	},
	// "every day" → 9am daily
	{
		pattern:  regexp.MustCompile(`(?i)^every day$`),
		cronExpr: "0 9 * * *",
		examples: []example{{"every day", "0 9 * * *"}},
	},
}

// Parse converts a natural language schedule or raw cron expression into
// a standard 5-field cron expression. Returns an error with examples if
// the input is not recognized.
func Parse(raw string) (string, error) {
	s := strings.TrimSpace(raw)

	// Try each natural language rule first.
	for _, r := range rules {
		matches := r.pattern.FindStringSubmatch(s)
		if matches == nil {
			continue
		}
		if r.cronExpr != "" {
			return r.cronExpr, nil
		}
		return r.handler(matches)
	}

	// Fall through: try treating it as a raw cron expression.
	if _, err := cron.ParseStandard(s); err == nil {
		return s, nil
	}

	msg := fmt.Sprintf("unrecognized schedule %q", s)
	if suggestion, ok := didYouMean(s); ok {
		msg += fmt.Sprintf("\n\ndid you mean %q?", suggestion)
	}
	return "", fmt.Errorf("%s\n\nExamples:\n%s", msg, exampleList())
}

// NextRun returns the next time a cron expression fires after the given time.
func NextRun(cronExpr string, from time.Time) (time.Time, error) {
	schedule, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression %q: %w", cronExpr, err)
	}
	return schedule.Next(from), nil
}

// parseDayAtTime handles "every <weekday> at <time>" patterns.
func parseDayAtTime(m []string) (string, error) {
	day := dayNumber(m[1])
	hour, min, err := parseHourMin(m[2], m[3], m[4])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d %d * * %d", min, hour, day), nil
}

// parseHourMin converts hour string, optional minute string, and am/pm into
// 24-hour hour and minute integers.
func parseHourMin(hourStr, minStr, ampm string) (hour, min int, err error) {
	hour, err = strconv.Atoi(hourStr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour %q", hourStr)
	}
	if minStr != "" {
		min, err = strconv.Atoi(minStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid minutes %q", minStr)
		}
	}

	ampm = strings.ToLower(ampm)
	if ampm == "pm" && hour != 12 {
		hour += 12
	} else if ampm == "am" && hour == 12 {
		hour = 0
	}

	if hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour %d", hour)
	}
	if min < 0 || min > 59 {
		return 0, 0, fmt.Errorf("invalid minutes %d", min)
	}
	return hour, min, nil
}

// dayNumber converts a weekday name to its cron number (0=Sunday, 1=Monday...).
func dayNumber(day string) int {
	switch strings.ToLower(day) {
	case "sunday":
		return 0
	case "monday":
		return 1
	case "tuesday":
		return 2
	case "wednesday":
		return 3
	case "thursday":
		return 4
	case "friday":
		return 5
	case "saturday":
		return 6
	}
	return 0
}

// didYouMean returns the closest rule example to s if it is within edit
// distance 3, along with true. Returns ("", false) if nothing is close enough.
// Candidates are derived from rule examples — the single source of truth for
// all recognized schedule phrases.
func didYouMean(s string) (string, bool) {
	const maxDist = 3
	best := ""
	bestDist := maxDist + 1
	lower := strings.ToLower(s)
	for _, r := range rules {
		for _, ex := range r.examples {
			if d := levenshtein(lower, ex.input); d < bestDist {
				bestDist = d
				best = ex.input
			}
		}
	}
	if bestDist <= maxDist {
		return best, true
	}
	return "", false
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	row := make([]int, lb+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= la; i++ {
		prev := i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			next := min(min(row[j]+1, prev+1), row[j-1]+cost)
			row[j-1] = prev
			prev = next
		}
		row[lb] = prev
	}
	return row[lb]
}

// exampleList returns a formatted list of example schedules for error messages.
// Entries are derived from rule examples — adding a new rule automatically
// includes it here.
func exampleList() string {
	const inputWidth = 24 // wide enough for the longest example input with quotes
	var lines []string
	for _, r := range rules {
		for _, ex := range r.examples {
			quoted := `"` + ex.input + `"`
			lines = append(lines, fmt.Sprintf("  %-*s  →  %s", inputWidth, quoted, ex.cron))
		}
	}
	lines = append(lines, fmt.Sprintf("  %-*s  →  raw cron expression", inputWidth, "0 2 * * *"))
	return strings.Join(lines, "\n")
}

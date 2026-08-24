package appspec

import (
	"fmt"
	"strconv"
	"strings"
)

// ValidateSchedule accepts five-field cron expressions (minute hour
// day-of-month month day-of-week) built from "*", "*/step", single values,
// ranges and comma lists. It intentionally stays dependency-free: the build
// plane cannot rely on flaky proxy fetches for a grammar this small.
func ValidateSchedule(expr string) error {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return fmt.Errorf("want 5 fields, got %d", len(fields))
	}
	domains := [][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}
	names := []string{"minute", "hour", "day-of-month", "month", "day-of-week"}
	for i, field := range fields {
		if fieldErr := checkField(field, domains[i][0], domains[i][1]); fieldErr != nil {
			return fmt.Errorf("%s field %q: %w", names[i], field, fieldErr)
		}
	}
	return nil
}

func checkField(field string, lo, hi int) error {
	for _, part := range strings.Split(field, ",") {
		if partErr := checkPart(part, lo, hi); partErr != nil {
			return partErr
		}
	}
	return nil
}

func checkPart(part string, lo, hi int) error {
	if part == "*" {
		return nil
	}
	if step, ok := strings.CutPrefix(part, "*/"); ok {
		n, nErr := strconv.Atoi(step)
		if nErr != nil || n < 1 {
			return fmt.Errorf("bad step %q", step)
		}
		return nil
	}
	if a, b, found := strings.Cut(part, "-"); found {
		loV, loErr := strconv.Atoi(a)
		hiV, hiErr := strconv.Atoi(b)
		if loErr != nil || hiErr != nil || loV > hiV || loV < lo || hiV > hi {
			return fmt.Errorf("bad range %q", part)
		}
		return nil
	}
	n, nErr := strconv.Atoi(part)
	if nErr != nil || n < lo || n > hi {
		return fmt.Errorf("value out of [%d,%d]", lo, hi)
	}
	return nil
}

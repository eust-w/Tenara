package metering

import (
	"errors"
	"testing"
	"time"
)

type fakeProm struct {
	cpuValue    float64
	cpuErr      error
	memValue    float64
	memErr      error
	seenQueries []string
}

func (f *fakeProm) Scalar(query string) (float64, error) {
	f.seenQueries = append(f.seenQueries, query)
	if f.cpuErr != nil && len(f.seenQueries) == 1 {
		return 0, f.cpuErr
	}
	if f.memErr != nil && len(f.seenQueries) == 2 {
		return 0, f.memErr
	}
	return f.cpuValue + f.memValue, nil
}

func TestSampleIntervalLocked(t *testing.T) {
	if SampleInterval != 5*time.Minute {
		t.Fatalf("interval = %s, want 5m", SampleInterval)
	}
}

func TestCPUQueryCarriesAppLabel(t *testing.T) {
	q := cpuQuery("acme", 5)
	for _, want := range []string{"app_id=", "acme", "container_cpu_usage_seconds_total"} {
		if !contains(q, want) {
			t.Fatalf("query missing %q: %s", want, q)
		}
	}
}

func TestMemQueryCarriesAppLabel(t *testing.T) {
	q := memQuery("acme")
	if !contains(q, "container_memory_working_set_bytes") || !contains(q, "acme") {
		t.Fatalf("mem query = %q", q)
	}
}

func TestCollectDegradesInsteadOfBlocking(t *testing.T) {
	prom := &fakeProm{cpuValue: 250, memErr: errors.New("prom down")}
	collector := NewCollector(prom, func() time.Time { return time.Unix(1700000000, 0) })

	record, errs := collector.Collect("acme")
	if len(errs) == 0 {
		t.Fatal("degraded sample must surface errors")
	}
	if record.AppID != "acme" || record.CPUMillicores != 250 || record.MemoryBytes != 0 {
		t.Fatalf("partial record = %+v", record)
	}
	if record.SampledAt.Unix() != 1700000000 {
		t.Fatalf("sampled at = %s", record.SampledAt)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

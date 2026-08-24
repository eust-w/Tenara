// Package metering samples per-app resource usage into usage_records (RB§29
// R10). Collection failures degrade to warnings and never block deployments.
package metering

import (
	"fmt"
	"time"
)

// SampleInterval is the collection cadence for the metering job.
const SampleInterval = 5 * time.Minute

// UsageRecord is one sampled row of per-app usage.
type UsageRecord struct {
	AppID         string
	SampledAt     time.Time
	CPUMillicores float64
	MemoryBytes   float64
	StorageBytes  float64
	BuildMinutes  float64
	DBSizeBytes   float64
}

// SampleError marks one failed dimension while others still succeed.
type SampleError struct {
	Err      error
	Resource string
}

func (e *SampleError) Error() string {
	return fmt.Sprintf("%s sampling failed: %v", e.Resource, e.Err)
}

func (e *SampleError) Unwrap() error {
	return e.Err
}

// PromClient abstracts the metrics backend used by the samplers.
type PromClient interface {
	Scalar(query string) (float64, error)
}

func cpuQuery(appID string, windowMinutes int) string {
	return fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{app_id=%q}[%dm])) * 1000`,
		appID, windowMinutes)
}

func memQuery(appID string) string {
	return fmt.Sprintf(`sum(container_memory_working_set_bytes{app_id=%q})`, appID)
}

// Collector samples every usage dimension for one application.
type Collector struct {
	prom PromClient
	now  func() time.Time
}

func NewCollector(prom PromClient, now func() time.Time) *Collector {
	return &Collector{prom: prom, now: now}
}

// Collect gathers all dimensions; individual sampler failures degrade the
// affected fields to zero and are reported without aborting the rest.
func (c *Collector) Collect(appID string) (UsageRecord, []error) {
	record := UsageRecord{AppID: appID, SampledAt: c.now()}
	var errs []error

	if cpu, err := c.prom.Scalar(cpuQuery(appID, 5)); err != nil {
		errs = append(errs, &SampleError{Resource: "cpu", Err: err})
	} else {
		record.CPUMillicores = cpu
	}
	if mem, err := c.prom.Scalar(memQuery(appID)); err != nil {
		errs = append(errs, &SampleError{Resource: "memory", Err: err})
	} else {
		record.MemoryBytes = mem
	}

	if len(errs) > 0 {
		return record, errs
	}
	return record, nil
}

// BuildMinutes converts seconds of builder usage into minutes.
func BuildMinutes(seconds float64) float64 {
	return seconds / 60
}

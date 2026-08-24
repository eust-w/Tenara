package repair

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type fakeSource struct {
	calls     int
	failFirst int
	genErr    error
}

func (f *fakeSource) GeneratePatch(_ context.Context, _ DiagnosticsBundle) (Patch, error) {
	f.calls++
	if f.calls <= f.failFirst {
		return Patch{}, f.genErr
	}
	return Patch{Description: "wire DATABASE_URL", Files: map[string][]byte{
		".tenara/env": []byte("DATABASE_URL=$(MONGO_URL)"),
	}}, nil
}

type fakeAuditor struct {
	records []int
	errAt   int // 0 = never fail
	forced  error
}

func (a *fakeAuditor) RecordAttempt(_ context.Context, _ string, n int, _ Patch, _ error) error {
	a.records = append(a.records, n)
	if a.errAt == n {
		return a.forced
	}
	return nil
}

type countingApply struct {
	okFrom int
	calls  int
}

func (c *countingApply) Apply(_ context.Context, _ Patch) error {
	c.calls++
	if c.calls < c.okFrom {
		return fmt.Errorf("verify still red (%d)", c.calls)
	}
	return nil
}

func TestCanTriggerHardGate(t *testing.T) {
	for n := 1; n <= MaxAttempts; n++ {
		if err := CanTrigger(n); err != nil {
			t.Fatalf("attempt %d must be allowed: %v", n, err)
		}
	}
	if err := CanTrigger(MaxAttempts + 1); !errors.Is(err, ErrAttemptsExhausted) {
		t.Fatalf("fourth trigger must refuse: %v", err)
	}
	if err := CanTrigger(0); err == nil || errors.Is(err, ErrAttemptsExhausted) {
		t.Fatalf("zero must be range error: %v", err)
	}
}

func TestLoopRepairsWithinThreeAttempts(t *testing.T) {
	src := &fakeSource{failFirst: 2, genErr: errors.New("transient llm hiccup")}
	// Generation fails twice; the third attempt's first apply call goes green.
	app := &countingApply{okFrom: 1}
	aud := &fakeAuditor{}
	l := &Loop{Source: src, Auditor: aud, Apply: app.Apply}

	b := DiagnosticsBundle{AppID: "a1", Summary: "missing env DATABASE_URL"}
	if runErr := l.Run(context.Background(), "a1", b); runErr != nil {
		t.Fatalf("run: %v", runErr)
	}
	if src.calls != 3 || app.calls != 1 {
		t.Fatalf("calls src=%d apply=%d", src.calls, app.calls)
	}
	if len(aud.records) != 3 {
		t.Fatalf("audit records = %v", aud.records)
	}
}

func TestLoopForbidsFourthTrigger(t *testing.T) {
	src := &fakeSource{} // always yields a patch...
	app := &countingApply{okFrom: 99}
	aud := &fakeAuditor{}
	l := &Loop{Source: src, Auditor: aud, Apply: app.Apply}

	err := l.Run(context.Background(), "a1",
		DiagnosticsBundle{AppID: "a1", Summary: "crashloop"})
	if !errors.Is(err, ErrAttemptsExhausted) {
		t.Fatalf("want exhausted, got %v", err)
	}
	if len(aud.records) != MaxAttempts+1 { // three real tries + refusal entry
		t.Fatalf("records = %v", aud.records)
	}
	if src.calls != MaxAttempts {
		t.Fatalf("patch source fired %d times; fourth must be forbidden", src.calls)
	}
}

func TestLoopSurfacesAuditFailure(t *testing.T) {
	boom := errors.New("audit sink down")
	aud := &fakeAuditor{errAt: 1, forced: boom}
	l := &Loop{
		Source: &fakeSource{}, Auditor: aud,
		Apply: (&countingApply{okFrom: 1}).Apply,
	}
	if err := l.Run(context.Background(), "a1",
		DiagnosticsBundle{AppID: "a1"}); !errors.Is(err, boom) {
		t.Fatalf("audit error must surface: %v", err)
	}
}

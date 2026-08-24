package appenv

import (
	"encoding/json"
	"testing"
)

func cronFixture() ServiceInput {
	return ServiceInput{
		Name: "jobs", Image: "repo/jobs@sha256:aa", Schedule: "*/5 * * * *",
	}
}

func decode(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	raw, mErr := json.Marshal(m)
	if mErr != nil {
		t.Fatal(mErr)
	}
	var doc map[string]any
	if uErr := json.Unmarshal(raw, &doc); uErr != nil {
		t.Fatal(uErr)
	}
	return doc
}

func TestRenderCronJobShape(t *testing.T) {
	doc, err := RenderCronJob("app1", "prod", "ns1", cronFixture())
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, doc)
	if m["apiVersion"] != "batch/v1" || m["kind"] != "CronJob" {
		t.Fatalf("bad gvk: %v/%v", m["apiVersion"], m["kind"])
	}
	spec := m["spec"].(map[string]any)
	if spec["schedule"] != "*/5 * * * *" {
		t.Fatalf("schedule lost: %v", spec["schedule"])
	}
	if spec["concurrencyPolicy"] != "Forbid" {
		t.Fatalf("policy = %v", spec["concurrencyPolicy"])
	}
	tmpl := spec["jobTemplate"].(map[string]any)["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
	if tmpl["automountServiceAccountToken"] != false {
		t.Fatal("automount hardening lost")
	}
	c0 := tmpl["containers"].([]any)[0].(map[string]any)
	sc := c0["securityContext"].(map[string]any)
	if sc["readOnlyRootFilesystem"] != true {
		t.Fatal("readOnlyRootFilesystem lost")
	}
	if _, hasProbes := c0["readinessProbe"]; hasProbes {
		t.Fatal("cron jobs must not carry TCP probes")
	}
}

func TestRenderCronJobGuards(t *testing.T) {
	bad := cronFixture()
	bad.Image = "repo/jobs:latest"
	if _, err := RenderCronJob("a", "p", "n", bad); err == nil {
		t.Fatal(":latest must be refused")
	}
	noSchedule := cronFixture()
	noSchedule.Schedule = ""
	if _, err := RenderCronJob("a", "p", "n", noSchedule); err == nil {
		t.Fatal("missing schedule must fail")
	}
}

func TestRenderHPAShape(t *testing.T) {
	svc := ServiceInput{Name: "web", Image: "r/i@sha256:a", Replicas: 2}
	doc := decode(t, RenderHPA("ns1", svc, 6))
	spec := doc["spec"].(map[string]any)
	ref := spec["scaleTargetRef"].(map[string]any)
	if ref["kind"] != "Deployment" || ref["name"] != "web" {
		t.Fatalf("bad target: %v", ref)
	}
	if spec["minReplicas"] != float64(2) || spec["maxReplicas"] != float64(6) {
		t.Fatalf("replicas = %v..%v", spec["minReplicas"], spec["maxReplicas"])
	}
	cpu := spec["metrics"].([]any)[0].(map[string]any)["resource"].(map[string]any)
	target := cpu["target"].(map[string]any)
	if target["type"] != "Utilization" || target["averageUtilization"] != float64(70) {
		t.Fatalf("cpu target = %v", target)
	}
}

func TestRenderHPAClampsQuota(t *testing.T) {
	svc := ServiceInput{Name: "web", Image: "r/i@sha256:a"}
	doc := decode(t, RenderHPA("ns1", svc, 0)) // degenerate quota clamps to min
	spec := doc["spec"].(map[string]any)
	if spec["maxReplicas"] != float64(1) {
		t.Fatalf("clamp = %v", spec["maxReplicas"])
	}
}

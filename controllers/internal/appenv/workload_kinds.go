package appenv

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
)

// Workload kinds beyond the web Deployment (todo91 / D2-P2-4): cron maps to
// a batch/v1 CronJob and every service can gain an autoscaling/v2 HPA capped
// by its plan quota. Typed batch/autoscaling imports stay deferred until the
// module cache fetches them reliably, matching the httproute.go precedent.

const (
	// cronConcurrencyPolicy prevents overlapping runs of one schedule.
	cronConcurrencyPolicy = "Forbid"
	// hpaCPUUtilization is the design-doc scaling target (70%).
	hpaCPUUtilization = int32(70)
)

func toUnstructuredMap(v any) (map[string]any, error) {
	m, convErr := runtime.DefaultUnstructuredConverter.ToUnstructured(v)
	if convErr != nil {
		return nil, fmt.Errorf("convert %T: %w", v, convErr)
	}
	return m, nil
}

// RenderCronJob renders one hardened batch/v1 CronJob for a cron service;
// digest-pinned images are enforced exactly like web deployments and the job
// template reuses the RB§15 pod hardening minus TCP probes.
func RenderCronJob(
	appID, env, namespace string, s ServiceInput,
) (map[string]any, error) {
	if renderErr := RequireDigestImage(s.Image); renderErr != nil {
		return nil, renderErr
	}
	if s.Schedule == "" { // grammar ownership: appspec.ValidateSchedule
		return nil, errors.New("cron service requires a schedule")
	}
	ps := restrictedPodSpec(s, false)
	psMap, convErr := toUnstructuredMap(&ps)
	if convErr != nil {
		return nil, convErr
	}
	return map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "CronJob",
		"metadata": map[string]any{
			"name":      s.Name,
			"namespace": namespace,
			"labels": map[string]any{
				LabelManagedBy: LabelManagedVal,
				LabelAppID:     appID,
				LabelEnv:       env,
				"app":          s.Name,
			},
		},
		"spec": map[string]any{
			"schedule":                   s.Schedule,
			"concurrencyPolicy":          cronConcurrencyPolicy,
			"successfulJobsHistoryLimit": 1,
			"failedJobsHistoryLimit":     3,
			"jobTemplate": map[string]any{
				"spec": map[string]any{
					"template": map[string]any{"spec": psMap},
				},
			},
		},
	}, nil
}

// RenderHPA renders an autoscaling/v2 HorizontalPodAutoscaler targeting the
// service Deployment at the design-doc 70% CPU utilization, clamped to the
// plan quota ceiling.
func RenderHPA(namespace string, s ServiceInput, quotaMaxReplicas int32) map[string]any {
	minReplicas := s.Replicas
	if minReplicas < 1 {
		minReplicas = 1
	}
	maxReplicas := quotaMaxReplicas
	if maxReplicas < minReplicas {
		maxReplicas = minReplicas
	}
	return map[string]any{
		"apiVersion": "autoscaling/v2",
		"kind":       "HorizontalPodAutoscaler",
		"metadata": map[string]any{
			"name":      "hpa-" + s.Name,
			"namespace": namespace,
			"labels": map[string]any{
				LabelManagedBy: LabelManagedVal,
				"app":          s.Name,
			},
		},
		"spec": map[string]any{
			"scaleTargetRef": map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"name":       s.Name,
			},
			"minReplicas": minReplicas,
			"maxReplicas": maxReplicas,
			"metrics": []any{map[string]any{
				"type": "Resource",
				"resource": map[string]any{
					"name": "cpu",
					"target": map[string]any{
						"type":               "Utilization",
						"averageUtilization": hpaCPUUtilization,
					},
				},
			}},
		},
	}
}

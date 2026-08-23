package build

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	buildkitImage      = "moby/buildkit:master-rootless"
	flagNoProcessSbox  = "--oci-worker-no-process-sandbox"
	buildkitDataMount  = "/home/user/.local/share/buildkit"
	freeTierTimeoutSec = int64(600)
	builderRunAsUID    = int64(1000)
)

func boolPtr(b bool) *bool    { return &b }
func int64Ptr(i int64) *int64 { return &i }
func int32Ptr(i int32) *int32 { return &i }

// BuilderPod returns the rootless BuildKit builder Pod template (RB-12 R4).
func BuilderPod(
	name, namespace string,
	labels map[string]string,
) *corev1.Pod {
	cpuLimit := resource.MustParse("2")
	memLimit := resource.MustParse("4Gi")
	ephLimit := resource.MustParse("8Gi")

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                corev1.RestartPolicyNever,
			AutomountServiceAccountToken: boolPtr(false),
			NodeSelector:                 map[string]string{"tenara.io/role": "build"},
			Tolerations: []corev1.Toleration{{
				Key:      "tenara.io/role",
				Operator: corev1.TolerationOpEqual,
				Value:    "build",
				Effect:   corev1.TaintEffectNoSchedule,
			}},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  int64Ptr(builderRunAsUID),
				RunAsGroup: int64Ptr(builderRunAsUID),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeUnconfined,
				},
			},
			ActiveDeadlineSeconds: int64Ptr(freeTierTimeoutSec),
			Containers: []corev1.Container{{
				Name:            "buildkitd",
				Image:           buildkitImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Args:            []string{flagNoProcessSbox},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "buildkit-state",
					MountPath: "/home/user/.local/share/buildkit",
				}},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{ //nolint:exhaustive // partial resource set by design
						corev1.ResourceCPU:              cpuLimit,
						corev1.ResourceMemory:           memLimit,
						corev1.ResourceEphemeralStorage: ephLimit,
					},
				},
			}},
		},
	}
}

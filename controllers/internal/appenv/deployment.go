package appenv

import (
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ServiceInput is the minimal per-service view required to render a tenant
// Deployment.
type ServiceInput struct {
	Name      string
	Image     string
	Isolation IsolationLevel
	Schedule  string
	Port      int32
	Replicas  int32
}

// RequireDigestImage enforces digest-pinned references only (R3); :latest is
// rejected outright.
func RequireDigestImage(image string) error {
	if image == "" {
		return errors.New("empty image reference")
	}
	if strings.Contains(image, ":latest") || strings.Contains(image, "@latest") {
		return fmt.Errorf("latest tag is forbidden: %q", image)
	}
	if !strings.Contains(image, "@sha256:") {
		return fmt.Errorf("image %q is not digest-pinned", image)
	}
	return nil
}

func tcpProbe(port int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(port)},
		},
	}
}

// restrictedPodSpec assembles the RB§15-hardened pod spec shared by web
// Deployments and CronJob job templates; TCP ports/probes attach only when
// withProbes is set (cron workloads speak schedules, not HTTP).
func restrictedPodSpec(s ServiceInput, withProbes bool) corev1.PodSpec {
	container := corev1.Container{
		Name:  s.Name,
		Image: s.Image,
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             boolPtr(true),
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(true),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
	}
	if withProbes {
		container.Ports = []corev1.ContainerPort{{ContainerPort: s.Port}}
		container.ReadinessProbe = tcpProbe(s.Port)
		container.LivenessProbe = tcpProbe(s.Port)
	}
	podSpec := corev1.PodSpec{
		AutomountServiceAccountToken: boolPtr(false),
		Containers:                   []corev1.Container{container},
	}
	if s.Isolation == IsolationDedicated {
		ApplyDedicatedScheduling(&podSpec)
	} else {
		ApplyTenantScheduling(&podSpec)
	}
	ApplySandboxClass(&podSpec, s.Isolation)
	return podSpec
}

// RenderDeployment renders one RB§15-restricted Deployment for a service;
// non-digest images are refused before anything is built.
func RenderDeployment(appID, env, namespace string, s ServiceInput) (*appsv1.Deployment, error) {
	if renderErr := RequireDigestImage(s.Image); renderErr != nil {
		return nil, renderErr
	}

	replicas := s.Replicas
	if replicas < 1 {
		replicas = 1
	}

	labels := map[string]string{
		LabelManagedBy: LabelManagedVal,
		LabelAppID:     appID,
		LabelEnv:       env,
		"app":          s.Name,
	}

	podSpec := restrictedPodSpec(s, true)
	if schedErr := EnsureNoCrossPoolToleration(&podSpec); schedErr != nil {
		return nil, schedErr
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: s.Name, Namespace: namespace, Labels: labels},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": s.Name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec,
			},
		},
	}, nil
}

// RenderDeployments renders every service, failing fast at the first offender.
func RenderDeployments(appID, env, namespace string, svcs []ServiceInput) ([]*appsv1.Deployment, error) {
	out := make([]*appsv1.Deployment, 0, len(svcs))
	for _, s := range svcs {
		d, err := RenderDeployment(appID, env, namespace, s)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

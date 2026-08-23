package build

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func sampleBuild() *Build {
	return &Build{
		ObjectMeta: metav1.ObjectMeta{Name: "b1", Namespace: "tenara-build", UID: "uid-1"},
		Spec: BuildSpec{
			AppID: "app-1",
			Git: GitSource{
				URL:      "https://github.com/acme/widget.git",
				SHA:      "abc1234567890",
				TokenRef: "gh-secret",
			},
		},
	}
}

func containerText(c corev1.Container) string {
	parts := append([]string{}, c.Command...)
	for _, e := range c.Env {
		parts = append(parts, e.Name, e.Value)
	}
	return strings.Join(parts, "\n")
}

func TestEphemeralTokenSecretShape(t *testing.T) {
	b := sampleBuild()
	s := EphemeralTokenSecret(b, []byte("tok"))

	if s.Name != "b1-git-token" || s.Namespace != "tenara-build" {
		t.Fatalf("name/ns = %s/%s", s.Name, s.Namespace)
	}
	if len(s.Data["token"]) == 0 {
		t.Fatal("token data missing")
	}
	if len(s.OwnerReferences) != 1 {
		t.Fatalf("ownerRefs = %d", len(s.OwnerReferences))
	}
	o := s.OwnerReferences[0]
	if o.Kind != KindBuild || o.UID != types.UID("uid-1") || o.Controller == nil || !*o.Controller {
		t.Fatalf("ownerRef = %+v", o)
	}
}

func TestCloneInitContainerSecurity(t *testing.T) {
	b := sampleBuild()
	c := CloneInitContainer(b)

	txt := containerText(c)
	if strings.Contains(txt, "ghp_") {
		t.Fatal("plaintext token pattern leaked into command/env")
	}

	script := c.Command[len(c.Command)-1]
	for _, want := range []string{"--depth 1", "$GIT_SHA", "$GIT_URL", tokenMountPath + "/" + tokenFileName} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q", want)
		}
	}

	var tokMount, wsMount bool
	for _, vm := range c.VolumeMounts {
		switch vm.MountPath {
		case tokenMountPath:
			tokMount = vm.ReadOnly
		case workspaceMount:
			wsMount = true
		}
	}
	if !tokMount || !wsMount {
		t.Fatalf("mounts: tokenRO=%v workspace=%v", tokMount, wsMount)
	}

	for _, e := range c.Env {
		switch e.Name {
		case "GIT_URL":
			if e.Value != b.Spec.Git.URL {
				t.Fatal("GIT_URL mismatch")
			}
		case "GIT_SHA":
			if e.Value != b.Spec.Git.SHA {
				t.Fatal("GIT_SHA mismatch")
			}
		}
	}
}

func TestAppendCloneStageWiring(t *testing.T) {
	b := sampleBuild()
	pod := BuilderPod("b1", "tenara-build", nil)
	AppendCloneStage(pod, b)

	if len(pod.Spec.InitContainers) != 1 || pod.Spec.InitContainers[0].Name != "git-clone" {
		t.Fatalf("initContainers = %+v", pod.Spec.InitContainers)
	}

	vols := map[string]bool{}
	for _, v := range pod.Spec.Volumes {
		vols[v.Name] = true
	}
	if !vols["b1-git-token"] || !vols[workspaceVolume] {
		t.Fatalf("volumes = %v", vols)
	}

	var bkd *corev1.Container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == "buildkitd" {
			bkd = &pod.Spec.Containers[i]
		}
	}
	if bkd == nil {
		t.Fatal("buildkitd missing")
	}
	found := false
	for _, vm := range bkd.VolumeMounts {
		if vm.Name == workspaceVolume && vm.MountPath == workspaceMount {
			found = true
		}
	}
	if !found {
		t.Fatal("buildkitd lacks workspace mount")
	}
}

func TestDeleteEphemeralTokenSecret(t *testing.T) {
	sch := runtime.NewScheme()
	if err := corev1.AddToScheme(sch); err != nil {
		t.Fatal(err)
	}
	name := EphemeralTokenSecretName("b1")
	c := fake.NewClientBuilder().WithScheme(sch).WithObjects(
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"}},
	).Build()

	DeleteEphemeralTokenSecret(context.Background(), c, "ns", "b1")

	var s corev1.Secret
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "ns", Name: name}, &s); err == nil {
		t.Fatal("secret still exists after delete")
	}
	DeleteEphemeralTokenSecret(context.Background(), c, "ns", "b1")
}

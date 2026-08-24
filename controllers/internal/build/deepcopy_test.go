package build

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func fixtureBuild() *Build {
	return &Build{
		TypeMeta:   metav1.TypeMeta{APIVersion: APIVersion, Kind: KindBuild},
		ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns", Labels: map[string]string{"l": "v"}, Annotations: map[string]string{"k": "v"}, Finalizers: []string{"f"}},
		Spec:       BuildSpec{AppID: "app", Env: "prod", Git: GitSource{URL: "https://git.test/r.git", SHA: "abc"}, Dockerfile: "Dockerfile"},
		Status:     BuildStatus{Phase: PhasePushed, ImageDigest: "sha256:a", Reason: "ok", Message: "m"},
	}
}

func TestBuildDeepCopyRoundTrip(t *testing.T) {
	orig := fixtureBuild()
	cp := orig.DeepCopy()
	if !reflect.DeepEqual(orig, cp) {
		t.Fatal("copy must equal original")
	}
	if _, ok := orig.DeepCopyObject().(*Build); !ok {
		t.Fatal("DeepCopyObject must return *Build")
	}
	cp.Spec.AppID = "mutated"
	if orig.Spec.AppID == "mutated" {
		t.Fatal("spec leaked")
	}
	cp.Spec.Git.SHA = "mutated"
	if orig.Spec.Git.SHA != "abc" {
		t.Fatal("git source leaked")
	}
	cp.Labels["l"] = "mutated"
	if orig.Labels["l"] != "v" {
		t.Fatal("labels leaked")
	}
}

func TestBuildListDeepCopy(t *testing.T) {
	orig := &BuildList{Items: []Build{*fixtureBuild(), *fixtureBuild()}}
	cp := orig.DeepCopy()
	if !reflect.DeepEqual(orig, cp) || len(cp.Items) != 2 {
		t.Fatal("list copy broken")
	}
	if orig.DeepCopyObject() == nil {
		t.Fatal("nil list object")
	}
	cp.Items[0].Spec.Env = "mutated"
	if orig.Items[0].Spec.Env != "prod" {
		t.Fatal("items leaked")
	}
}

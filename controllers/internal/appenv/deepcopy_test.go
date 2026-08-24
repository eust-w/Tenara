package appenv

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func fixtureAppEnv() *AppEnv {
	return &AppEnv{
		TypeMeta:   metav1.TypeMeta{APIVersion: APIVersion, Kind: Kind},
		ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns", Labels: map[string]string{"l": "v"}, Annotations: map[string]string{"k": "v"}, Finalizers: []string{"f"}},
		Spec:       AppEnvSpec{AppID: "app", Env: "prod", AppSpecRef: "r", DomainRefs: []string{"d1", "d2"}, QuotaTier: QuotaPro, Isolation: IsolationIsolated},
		Status:     AppEnvStatus{Namespace: "ns", Phase: "RUNNING", Reason: "ok", Message: "m"},
	}
}

func TestAppEnvDeepCopyRoundTrip(t *testing.T) {
	orig := fixtureAppEnv()
	cp := orig.DeepCopy()
	if !reflect.DeepEqual(orig, cp) {
		t.Fatal("copy must equal original")
	}
	if _, ok := orig.DeepCopyObject().(*AppEnv); !ok {
		t.Fatal("DeepCopyObject must return *AppEnv")
	}
	cp.Spec.AppID = "mutated"
	if orig.Spec.AppID == "mutated" {
		t.Fatal("spec leaked")
	}
	cp.Spec.DomainRefs[0] = "mutated"
	if orig.Spec.DomainRefs[0] != "d1" {
		t.Fatal("domain slice leaked")
	}
	cp.Labels["l"] = "mutated"
	if orig.Labels["l"] != "v" {
		t.Fatal("labels leaked")
	}
}

func TestAppEnvListDeepCopy(t *testing.T) {
	orig := &AppEnvList{Items: []AppEnv{*fixtureAppEnv(), *fixtureAppEnv()}}
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

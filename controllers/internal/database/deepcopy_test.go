package database

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func fixtureBinding() *DatabaseBinding {
	return &DatabaseBinding{
		TypeMeta:   metav1.TypeMeta{APIVersion: APIVersion, Kind: Kind},
		ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns", Labels: map[string]string{"l": "v"}, Annotations: map[string]string{"k": "v"}, Finalizers: []string{"f"}},
		Spec:       DatabaseBindingSpec{AppID: "app", Env: "prod", Kind: KindMongo},
		Status:     DatabaseBindingStatus{Phase: "READY", Reason: "ok", Message: "m"},
	}
}

func TestBindingDeepCopyRoundTrip(t *testing.T) {
	orig := fixtureBinding()
	cp := orig.DeepCopy()
	if !reflect.DeepEqual(orig, cp) {
		t.Fatal("copy must equal original")
	}
	if _, ok := orig.DeepCopyObject().(*DatabaseBinding); !ok {
		t.Fatal("DeepCopyObject must return *DatabaseBinding")
	}
	cp.Spec.AppID = "mutated"
	if orig.Spec.AppID == "mutated" {
		t.Fatal("spec leaked")
	}
	cp.Labels["l"] = "mutated"
	if orig.Labels["l"] != "v" {
		t.Fatal("labels leaked")
	}
}

func TestBindingListDeepCopy(t *testing.T) {
	orig := &DatabaseBindingList{Items: []DatabaseBinding{*fixtureBinding(), *fixtureBinding()}}
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

package cells

import (
	"errors"
	"reflect"
	"testing"
)

func TestRouteByAssignmentThenDefault(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register("cell-a", "https://a.dataplane", "cn-east", false); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("cell-b", "https://b.dataplane", "cn-north", true); err != nil {
		t.Fatal(err)
	}
	if err := reg.Assign("org-x", "cell-a"); err != nil {
		t.Fatal(err)
	}
	got, rerr := reg.RouteForOrg("org-x")
	if rerr != nil || got.Name != "cell-a" {
		t.Fatalf("assigned route = %+v %v", got, rerr)
	}
	got, rerr = reg.RouteForOrg("org-y")
	if rerr != nil || got.Name != "cell-b" {
		t.Fatalf("default route = %+v %v", got, rerr)
	}
}

func TestRegistryValidation(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register("", "ep", "", false); err == nil {
		t.Fatal("anonymous cell rejected")
	}
	if err := reg.Register("c", "", "", false); err == nil {
		t.Fatal("endpoint-less cell rejected")
	}
	if err := reg.Register("c", "ep", "", false); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("c", "ep2", "", false); !errors.Is(err, ErrDuplicateCell) {
		t.Fatalf("dup = %v", err)
	}
	if err := reg.Assign("o", "ghost"); !errors.Is(err, ErrUnknownCell) {
		t.Fatalf("unknown assign = %v", err)
	}
	regNoDefault := NewRegistry()
	if _, err := regNoDefault.RouteForOrg("any"); !errors.Is(err, ErrNoCell) {
		t.Fatalf("no-cell = %v", err)
	}
}

func TestFaultIsolationBlastRadius(t *testing.T) {
	homes := map[string]string{
		"app-a1": "cell-a", "app-a2": "cell-a",
		"app-b1": "cell-b",
	}
	got := AffectedApps(homes, "cell-b")
	want := []string{"app-b1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("blast radius = %v, want %v", got, want)
	}
	if len(AffectedApps(homes, "cell-z")) != 0 {
		t.Fatal("healthy cell must have empty radius")
	}
}

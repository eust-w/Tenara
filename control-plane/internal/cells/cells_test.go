package cells

import (
	"errors"
	"reflect"
	"testing"
)

func mustRegister(t *testing.T, r *Registry, s Spec) {
	t.Helper()
	if err := r.Register(s); err != nil {
		t.Fatalf("register %s: %v", s.Name, err)
	}
}

func TestRouteByAssignmentThenDefault(t *testing.T) {
	reg := NewRegistry()
	mustRegister(t, reg, Spec{
		Name: "cell-a", Cloud: "baidu-cce",
		Endpoint: "https://a.dataplane", Region: "cn-east",
	})
	mustRegister(t, reg, Spec{
		Name: "cell-b", Cloud: "baidu-cce",
		Endpoint: "https://b.dataplane", Region: "cn-north", Default: true,
	})
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
	if err := reg.Register(Spec{Endpoint: "ep"}); err == nil {
		t.Fatal("anonymous cell rejected")
	}
	if err := reg.Register(Spec{Name: "c"}); err == nil {
		t.Fatal("endpoint-less cell rejected")
	}
	if err := reg.Register(Spec{Name: "c", Cloud: "openstack", Endpoint: "ep"}); !errors.Is(err, ErrUnknownCloud) {
		t.Fatalf("unknown cloud = %v", err)
	}
	if err := reg.Register(Spec{Name: "c", Cloud: "baidu-cce", Endpoint: "ep"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(Spec{Name: "c", Cloud: "baidu-cce", Endpoint: "ep2"}); !errors.Is(err, ErrDuplicateCell) {
		t.Fatalf("dup = %v", err)
	}
	if err := reg.Assign("o", "ghost"); !errors.Is(err, ErrUnknownCell) {
		t.Fatalf("unknown assign = %v", err)
	}
	noDefault := NewRegistry()
	if _, err := noDefault.RouteForOrg("any"); !errors.Is(err, ErrNoCell) {
		t.Fatalf("no-cell = %v", err)
	}
}

func TestTargetForOrgAcrossClouds(t *testing.T) {
	reg := NewRegistry()
	mustRegister(t, reg, Spec{
		Name: "bj", Cloud: "baidu-cce",
		Endpoint: "https://bj.dp", Region: "cn-bj",
	})
	mustRegister(t, reg, Spec{
		Name: "hz", Cloud: "aliyun-ack",
		Endpoint: "https://hz.dp", Region: "cn-hz",
	})
	mustRegister(t, reg, Spec{
		Name: "gz", Cloud: "tencent-tke",
		Endpoint: "https://gz.dp", Region: "cn-gz",
	})
	mustRegister(t, reg, Spec{
		Name: "lab", Cloud: "selfhosted",
		Endpoint: "https://lab.dp", Default: true,
	})
	if err := reg.Assign("org-x", "gz"); err != nil {
		t.Fatal(err)
	}

	tgt, terr := reg.TargetForOrg("org-x")
	if terr != nil || tgt.Cloud != "tencent-tke" || tgt.Endpoint != "https://gz.dp" ||
		tgt.Region != "cn-gz" || tgt.CellName != "gz" {
		t.Fatalf("assigned target = %+v %v", tgt, terr)
	}
	tgt, terr = reg.TargetForOrg("org-y")
	if terr != nil || tgt.Cloud != "selfhosted" || tgt.CellName != "lab" {
		t.Fatalf("fallback target = %+v %v", tgt, terr)
	}
}

func TestFleetByCloudGrouping(t *testing.T) {
	reg := NewRegistry()
	mustRegister(t, reg, Spec{Name: "b1", Cloud: "baidu-cce", Endpoint: "e1"})
	mustRegister(t, reg, Spec{Name: "b0", Cloud: "baidu-cce", Endpoint: "e0"})
	mustRegister(t, reg, Spec{Name: "a1", Cloud: "aliyun-ack", Endpoint: "e2"})
	fleet := reg.FleetByCloud()
	want := map[string][]string{
		"baidu-cce":  {"b0", "b1"},
		"aliyun-ack": {"a1"},
	}
	if !reflect.DeepEqual(fleet, want) {
		t.Fatalf("fleet = %v, want %v", fleet, want)
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

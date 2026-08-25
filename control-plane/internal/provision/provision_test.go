package provision

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestBuildAppEnvShape(t *testing.T) {
	obj := BuildAppEnv(AppEnvInput{
		AppID: "uuid-1", Env: "prod", Name: "probe", QuotaTier: "pro", Isolation: "shared",
	})
	if obj["apiVersion"] != "tenara.io/v1" || obj["kind"] != "AppEnv" {
		t.Fatalf("gvk = %v/%v", obj["apiVersion"], obj["kind"])
	}
	spec := obj["spec"].(map[string]any)
	if spec["appId"] != "uuid-1" || spec["quotaTier"] != "pro" {
		t.Fatalf("spec = %v", spec)
	}
	raw, marshalErr := json.Marshal(obj)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(obj, back) {
		t.Fatal("json round-trip changed content")
	}
}

type recordingApplier struct{ got []Object }

func (r *recordingApplier) Apply(_ context.Context, obj Object) error {
	r.got = append(r.got, obj)
	return nil
}

func TestRecordingApplierContract(t *testing.T) {
	rec := &recordingApplier{}
	var ap Applier = rec
	obj := Object{"apiVersion": "tenara.io/v1", "kind": "AppEnv"}
	if err := ap.Apply(context.Background(), obj); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(rec.got) != 1 || !reflect.DeepEqual(rec.got[0], obj) {
		t.Fatalf("recorded = %v", rec.got)
	}
}

func TestBuildBuildShape(t *testing.T) {
	obj := BuildBuild(BuildInput{
		AppID: "uuid-1", Env: "prod", Name: "probe-b1",
		GitURL: "https://git.test/r.git", GitSHA: "abc123",
	})
	if obj["kind"] != "Build" || obj["apiVersion"] != "tenara.io/v1" {
		t.Fatalf("gvk wrong: %v", obj)
	}
	spec := obj["spec"].(map[string]any)
	git := spec["git"].(map[string]any)
	if git["url"] != "https://git.test/r.git" || git["sha"] != "abc123" {
		t.Fatalf("git = %v", git)
	}
	if _, has := spec["dockerfile"]; has {
		t.Fatal("empty dockerfile must be omitted")
	}
}

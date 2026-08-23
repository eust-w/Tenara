package cce

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"tenara/providers/types"
)

type fixtureResponse struct {
	status int
	body   string
	err    error
}

type fixtureDoer struct {
	responses map[string]fixtureResponse
	calls     []string
}

func (f *fixtureDoer) Do(ctx context.Context, cfg Config, method, path string, body []byte) (int, []byte, error) {
	key := method + " " + path
	f.calls = append(f.calls, key)
	if resp, ok := f.responses[key]; ok {
		return resp.status, []byte(resp.body), resp.err
	}
	return 0, nil, errors.New("no fixture for " + key)
}

func TestCreateClusterHappy(t *testing.T) {
	doer := &fixtureDoer{responses: map[string]fixtureResponse{
		"POST /v1/cluster": {status: 200, body: `{"clusterId":"c-77"}`},
	}}
	p := New(Config{Region: "bj", AccessKey: "ak", SecretKey: "sk"}, doer)

	id, err := p.CreateCluster(context.Background(), ClusterSpec{
		Name: "tenara", Version: "1.30", NodeCount: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "c-77" {
		t.Fatalf("id = %q", id)
	}
	if len(doer.calls) != 1 || doer.calls[0] != "POST /v1/cluster" {
		t.Fatalf("calls = %v", doer.calls)
	}
}

func TestRegionIsParameterized(t *testing.T) {
	a := Config{Region: "bj"}.endpoint()
	b := Config{Region: "gz"}.endpoint()
	if a == b {
		t.Fatal("regions must yield distinct endpoints")
	}
	if !strings.Contains(a, "bj") || !strings.Contains(b, "gz") {
		t.Fatalf("endpoints = %q / %q", a, b)
	}
	if strings.Contains(a, "{region}") {
		t.Fatal("placeholder must be substituted")
	}
}

func TestGetClusterErrorSurfacesUnavailable(t *testing.T) {
	doer := &fixtureDoer{responses: map[string]fixtureResponse{
		"GET /v1/cluster/c-9": {status: 500, body: `{"message":"boom"}`},
	}}
	p := New(Config{Region: "bj"}, doer)

	if _, err := p.GetCluster(context.Background(), "c-9"); !errors.Is(err, types.ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
}

func TestHealthzWrapsTransportFailure(t *testing.T) {
	doer := &fixtureDoer{responses: map[string]fixtureResponse{
		"GET /v1/cluster": {err: errors.New("dial refused")},
	}}
	p := New(Config{Region: "bj"}, doer)

	if err := p.Healthz(context.Background()); !errors.Is(err, types.ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
}

func TestTestSourcesContainNoRealEndpoints(t *testing.T) {
	src, rErr := os.ReadFile("provider_test.go")
	if rErr != nil {
		t.Fatal(rErr)
	}
	for _, banned := range []string{"http" + "://", "http" + "s://", "baidubce" + ".com"} {
		if strings.Contains(string(src), banned) {
			t.Fatalf("test source embeds real endpoint fragment %q", banned)
		}
	}
}

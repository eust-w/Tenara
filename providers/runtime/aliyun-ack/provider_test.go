package aliyunack

import (
	"context"
	"errors"
	"testing"

	"tenara/providers/types"
)

type fixtureResp struct {
	body   []byte
	status int
	err    error
}

type fakeDoer struct {
	resp   fixtureResp
	method string
	path   string
	body   []byte
}

func (f *fakeDoer) Do(_ context.Context, _ Config, method, path string, body []byte) (int, []byte, error) {
	f.method = method
	f.path = path
	f.body = body
	return f.resp.status, f.resp.body, f.resp.err
}

func TestEndpointTemplate(t *testing.T) {
	defaultCfg := Config{Region: "cn-hangzhou"}
	if got := defaultCfg.endpoint(); got != "https://cs.cn-hangzhou.aliyuncs.com" {
		t.Fatalf("endpoint = %q", got)
	}
	overrideCfg := Config{Region: "r", Endpoint: "https://override/{region}"}
	if got := overrideCfg.endpoint(); got != "https://override/r" {
		t.Fatalf("override = %q", got)
	}
}

func TestCreateClusterHappyPath(t *testing.T) {
	fd := &fakeDoer{resp: fixtureResp{status: 200, body: []byte(`{"clusterId":"ack-123"}`)}}
	p := New(Config{Region: "cn-hangzhou"}, fd)
	id, err := p.CreateCluster(context.Background(),
		ClusterSpec{Name: "demo", Version: "1.30", NodeCount: 3})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id != "ack-123" || fd.method != "POST" || fd.path != apiPrefix+"/clusters" {
		t.Fatalf("id=%q method=%s path=%s", id, fd.method, fd.path)
	}
}

func TestSentinelWrapsFailures(t *testing.T) {
	fd := &fakeDoer{resp: fixtureResp{err: errors.New("boom")}}
	p := New(Config{}, fd)
	if _, err := p.CreateCluster(context.Background(),
		ClusterSpec{Name: "d", Version: "v", NodeCount: 1}); !errors.Is(err, types.ErrUnavailable) {
		t.Fatalf("transport wrap = %v", err)
	}
	fd2 := &fakeDoer{resp: fixtureResp{status: 503}}
	p2 := New(Config{}, fd2)
	if err := p2.Healthz(context.Background()); !errors.Is(err, types.ErrUnavailable) {
		t.Fatalf("healthz wrap = %v", err)
	}
	if st, err := p2.GetCluster(context.Background(), "c1"); err == nil || st != "" {
		t.Fatalf("negative get = %q %v", st, err)
	}
}

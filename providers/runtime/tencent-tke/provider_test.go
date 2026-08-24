package tencentrke

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
	cfg := Config{Region: "ap-guangzhou"}
	if got := cfg.endpoint(); got != "https://tke.ap-guangzhou.tencentcloudapi.com" {
		t.Fatalf("endpoint = %q", got)
	}
}

func TestCreateClusterHappyPath(t *testing.T) {
	fd := &fakeDoer{resp: fixtureResp{status: 200, body: []byte(`{"clusterId":"tke-9"}`)}}
	p := New(Config{Region: "ap-guangzhou"}, fd)
	id, err := p.CreateCluster(context.Background(),
		ClusterSpec{Name: "demo", Version: "1.30", NodeCount: 2})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id != "tke-9" || fd.method != "POST" || fd.path != apiPrefix+"/clusters" {
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
	fd2 := &fakeDoer{resp: fixtureResp{status: 500}}
	p2 := New(Config{}, fd2)
	if _, err := p2.GetCluster(context.Background(), "c"); !errors.Is(err, types.ErrUnavailable) {
		t.Fatalf("status wrap = %v", err)
	}
}

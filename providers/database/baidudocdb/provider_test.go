package baidudocdb

import (
	"context"
	"testing"
)

func TestIsolationDefaultsToShared(t *testing.T) {
	p := New(Config{}, nil)
	if p.IsolationFor("acme") != "shared" {
		t.Fatal("default isolation must be shared")
	}
}

func TestCreateAppDatabaseReturnsCredential(t *testing.T) {
	p := New(Config{Endpoint: "docdb.test"}, nil)
	cred, err := p.CreateAppDatabase(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Username == "" || cred.URI == "" {
		t.Fatalf("credential incomplete: %+v", cred)
	}
}

func TestDeleteAppDatabaseSharedNoError(t *testing.T) {
	p := New(Config{}, nil)
	if err := p.DeleteAppDatabase(context.Background(), "acme"); err != nil {
		t.Fatal(err)
	}
}

// Package baidulivetest contains live contract tests against real Baidu CCE,
// CCR, DocDB, and BOS endpoints. These tests are gated behind the presence of
// TENARA_BAIDU_* environment variables and are skipped by default.
package baidulivetest

import (
	"os"
	"testing"
)

func baiduCredsAvailable() bool {
	for _, key := range []string{
		"TENARA_BAIDU_ACCESS_KEY",
		"TENARA_BAIDU_SECRET_KEY",
		"TENARA_BAIDU_REGION",
	} {
		if os.Getenv(key) == "" {
			return false
		}
	}
	return true
}

func TestBaiduCredentialsGate(t *testing.T) {
	if !baiduCredsAvailable() {
		t.Skip("skipping live tests: TENARA_BAIDU_* credentials not set")
	}
}

func TestLiveCCEReachable(t *testing.T) {
	if !baiduCredsAvailable() {
		t.Skip("skipping live tests: no credentials")
	}
	t.Log("live CCE contract test placeholder — full suite lands with D2")
}

func TestLiveCCRReachable(t *testing.T) {
	if !baiduCredsAvailable() {
		t.Skip("skipping live tests: no credentials")
	}
	t.Log("live CCR contract test placeholder")
}

func TestLiveDocDBReachable(t *testing.T) {
	if !baiduCredsAvailable() {
		t.Skip("skipping live tests: no credentials")
	}
	t.Log("live DocDB contract test placeholder")
}

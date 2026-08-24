package previews

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func sign(t *testing.T, secret string, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestDecidePRAction(t *testing.T) {
	cases := []struct {
		action string
		want   PREventAction
		merged bool
	}{
		{"opened", ActionOpen, false},
		{"reopened", ActionOpen, false},
		{"closed", ActionClose, true},
		{"closed", ActionClose, false},
		{"synchronize", ActionIgnore, false},
		{"labeled", ActionIgnore, false},
	}
	for _, tc := range cases {
		if got := DecidePRAction(tc.action, tc.merged); got != tc.want {
			t.Fatalf("%s/%v -> %q, want %q", tc.action, tc.merged, got, tc.want)
		}
	}
}

func TestPreviewEnvName(t *testing.T) {
	if got := PreviewEnvName(42); got != "preview-pr-42" {
		t.Fatalf("name = %q", got)
	}
}

func TestWebhookLifecycleOpenClose(t *testing.T) {
	secret := "whsec"
	var opened, closed []int
	h := &Handler{
		Secret: secret,
		Guard:  NewReplayGuard(time.Minute),
		Hooks: Hooks{
			OnOpen:  func(pr int) error { opened = append(opened, pr); return nil },
			OnClose: func(pr int) error { closed = append(closed, pr); return nil },
		},
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	post := func(delivery, action string, pr int) int {
		body := fmt.Sprintf(`{"action":%q,"number":%d,"pull_request":{"merged":false}}`, action, pr)
		req, _ := http.NewRequest(http.MethodPost, srv.URL,
			bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Hub-Signature-256", sign(t, secret, []byte(body)))
		req.Header.Set("X-GitHub-Delivery", delivery)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if code := post("d1", "opened", 7); code != http.StatusAccepted {
		t.Fatalf("open = %d", code)
	}
	if len(opened) != 1 || opened[0] != 7 {
		t.Fatalf("opened = %v", opened)
	}
	if code := post("d2", "synchronize", 7); code != http.StatusAccepted {
		t.Fatalf("sync = %d", code)
	}
	if len(opened) != 1 {
		t.Fatal("synchronize must not reopen")
	}
	if code := post("d3", "closed", 7); code != http.StatusAccepted {
		t.Fatalf("close = %d", code)
	}
	if len(closed) != 1 || closed[0] != 7 {
		t.Fatalf("closed = %v", closed)
	}
}

func TestWebhookRejectsTamperAndReplay(t *testing.T) {
	secret := "whsec"
	h := &Handler{Secret: secret, Guard: NewReplayGuard(time.Minute)}
	srv := httptest.NewServer(h)
	defer srv.Close()
	body := []byte(`{"action":"opened","number":1,"pull_request":{"merged":false}}`)

	req, _ := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")
	req.Header.Set("X-GitHub-Delivery", "x1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tampered = %d, want 401", resp.StatusCode)
	}

	signed := func(id string) int {
		r2, _ := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(body))
		r2.Header.Set("X-Hub-Signature-256", sign(t, secret, body))
		r2.Header.Set("X-GitHub-Delivery", id)
		rs, dErr := http.DefaultClient.Do(r2)
		if dErr != nil {
			t.Fatal(dErr)
		}
		defer rs.Body.Close()
		return rs.StatusCode
	}
	if code := signed("same"); code != http.StatusAccepted {
		t.Fatalf("first delivery = %d", code)
	}
	if code := signed("same"); code != http.StatusConflict {
		t.Fatalf("replay = %d, want 409", code)
	}
}

func TestGHCommenterPostsBody(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(r.Body)
		gotBody = buf.String()
		w.WriteHeader(http.StatusCreated)
	}))
	defer ts.Close()

	c := &GHCommenter{BaseURL: ts.URL, Token: "tkn"}
	if err := c.PostPreviewURL(context.Background(), "acme/widget", 9, "https://pr9.example"); err != nil {
		t.Fatal(err)
	}
	wantPath := "/repos/acme/widget/issues/9/comments"
	if gotPath != wantPath {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer tkn" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if !bytes.Contains([]byte(gotBody), []byte("pr9.example")) ||
		!bytes.Contains([]byte(gotBody), []byte("preview-pr-9")) {
		t.Fatalf("comment missing url/env: %q", gotBody)
	}
}

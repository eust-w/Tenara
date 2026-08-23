package securityevent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validEvent() Event {
	return Event{
		EventCode: CodeLoginFailed,
		ActorType: "user",
		ActorID:   "u1",
		OrgID:     "o1",
		Detail:    "5 failed attempts",
		At:        time.Unix(1700000000, 0),
	}
}

func TestValidateAcceptsWellFormed(t *testing.T) {
	if err := Validate(validEvent()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsUnknownCode(t *testing.T) {
	e := validEvent()
	e.EventCode = "made.up.code"
	if err := Validate(e); err == nil || !strings.Contains(err.Error(), "unknown event code") {
		t.Fatalf("want unknown-code error, got %v", err)
	}
}

func TestValidateRejectsMissingTimestampAndBadActor(t *testing.T) {
	e := validEvent()
	e.At = time.Time{}
	if err := Validate(e); err == nil {
		t.Fatal("missing timestamp must be rejected")
	}
	e = validEvent()
	e.ActorType = "robot"
	if err := Validate(e); err == nil {
		t.Fatal("bad actor type must be rejected")
	}
}

func TestDashboardJSONValidAndDatasourceWired(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", "observability", "dashboards", "security.json"))
	if err != nil {
		t.Fatal(err)
	}
	var panel struct {
		Title      string `json:"title"`
		Datasource struct {
			UID string `json:"uid"`
		} `json:"datasource"`
		Panels []struct {
			Type string `json:"type"`
		} `json:"panels"`
	}
	if err := json.Unmarshal(body, &panel); err != nil {
		t.Fatal(err)
	}
	if panel.Title == "" || panel.Datasource.UID == "" || len(panel.Panels) < 2 {
		t.Fatalf("dashboard incomplete: %+v", panel)
	}
}

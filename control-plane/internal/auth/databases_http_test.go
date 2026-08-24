package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestDatabasesHTTP(t *testing.T) {
	ts := newTestServer(t)
	jwt := registerVerifiedUser(t, ts, "t21")
	stamp := timeNowUnix()
	appID := createNamedApp(t, ts, jwt, fmt.Sprintf("dbhttp-%d", stamp))

	t.Run("create then merge over wire", func(t *testing.T) {
		code1, body1 := authedJSON(t, http.MethodPost,
			ts.URL+"/v1/apps/"+appID+"/databases", jwt,
			map[string]string{"isolation": "shared"})
		if code1 != http.StatusCreated {
			t.Fatalf("first = %d body=%s", code1, body1)
		}
		code2, body2 := authedJSON(t, http.MethodPost,
			ts.URL+"/v1/apps/"+appID+"/databases", jwt,
			map[string]string{"isolation": "shared"})
		if code2 != http.StatusCreated {
			t.Fatalf("repeat = %d", code2)
		}
		var first, second struct {
			Database struct {
				ID string `json:"id"`
			} `json:"database"`
		}
		if decodeErr := json.Unmarshal(body1, &first); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if decodeErr := json.Unmarshal(body2, &second); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if first.Database.ID == "" || first.Database.ID != second.Database.ID {
			t.Fatalf("ids differ/empty: %q vs %q", first.Database.ID, second.Database.ID)
		}
	})
}

func TestDatabasesMultiKindHTTP(t *testing.T) {
	ts := newTestServer(t)
	jwt := registerVerifiedUser(t, ts, "t22")
	stamp := timeNowUnix()
	appID := createNamedApp(t, ts, jwt, fmt.Sprintf("dbmulti-%d", stamp))

	t.Run("redis and storage kinds are distinct bindings", func(t *testing.T) {
		post := func(kind string) (int, []byte) {
			return authedJSON(t, http.MethodPost,
				ts.URL+"/v1/apps/"+appID+"/databases", jwt,
				map[string]string{"kind": kind})
		}
		codeR, bodyR := post("redis")
		if codeR != http.StatusCreated {
			t.Fatalf("redis = %d body=%s", codeR, bodyR)
		}
		var red struct {
			Database struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"database"`
		}
		if decodeErr := json.Unmarshal(bodyR, &red); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if red.Database.Type != "redis" || red.Database.ID == "" {
			t.Fatalf("redis row wrong: %+v", red.Database)
		}
		codeS, bodyS := post("storage")
		if codeS != http.StatusCreated {
			t.Fatalf("storage = %d", codeS)
		}
		var sto struct {
			Database struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"database"`
		}
		if decodeErr := json.Unmarshal(bodyS, &sto); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if sto.Database.Type != "storage" || sto.Database.ID == red.Database.ID {
			t.Fatalf("storage must be its own row: %+v vs %+v", sto.Database, red.Database)
		}
		codeR2, bodyR2 := post("redis")
		if codeR2 != http.StatusCreated {
			t.Fatalf("redis repeat = %d", codeR2)
		}
		var red2 struct {
			Database struct {
				ID string `json:"id"`
			} `json:"database"`
		}
		if decodeErr := json.Unmarshal(bodyR2, &red2); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if red2.Database.ID != red.Database.ID {
			t.Fatalf("redis repeat must merge: %q vs %q", red2.Database.ID, red.Database.ID)
		}
	})

	t.Run("unsupported kind rejected with 400", func(t *testing.T) {
		codeX, bodyX := authedJSON(t, http.MethodPost,
			ts.URL+"/v1/apps/"+appID+"/databases", jwt,
			map[string]string{"kind": "oracle"})
		if codeX != http.StatusBadRequest {
			t.Fatalf("oracle = %d body=%s", codeX, bodyX)
		}
	})
}

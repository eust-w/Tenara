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

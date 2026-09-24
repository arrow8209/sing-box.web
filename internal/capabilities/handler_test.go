package capabilities

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerReturnsPhaseOneCapabilities(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewHandler(PhaseOne).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d", recorder.Code)
	}
	var response Response
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response != PhaseOne {
		t.Fatalf("response=%+v", response)
	}
	if response.Runtime || !response.Traffic || !response.ProxyGroups || !response.Connections {
		t.Fatalf("traffic, connections and proxy groups capabilities should be enabled: %+v", response)
	}
	if !response.Subscriptions || !response.InboundResources || !response.Application {
		t.Fatalf("resource capabilities must be enabled: %+v", response)
	}
}

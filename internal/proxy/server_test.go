package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aegis-proxy/aegis/internal/config"
	"github.com/aegis-proxy/aegis/internal/testhelpers"
)

func TestHealthEndpointReturnsOK(t *testing.T) {
	server := NewProxyServer(config.DefaultConfig(), nil, nil, nil)

	recorder := httptest.NewRecorder()
	server.handleHealth(recorder, testhelpers.NewTestRequest(http.MethodGet, "/health", nil))

	resp := recorder.Result()
	defer resp.Body.Close()
	testhelpers.AssertStatusCode(t, resp, http.StatusOK)
	testhelpers.AssertJSONField(t, recorder.Body.Bytes(), "status", "ok")
}

func TestModelsEndpointReturnsValidJSON(t *testing.T) {
	server := NewProxyServer(config.DefaultConfig(), nil, nil, nil)

	recorder := httptest.NewRecorder()
	server.handleListModels(recorder, testhelpers.NewTestRequest(http.MethodGet, "/v1/models", nil))

	resp := recorder.Result()
	defer resp.Body.Close()
	testhelpers.AssertStatusCode(t, resp, http.StatusOK)
	testhelpers.AssertJSONField(t, recorder.Body.Bytes(), "object", "list")

	var body struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal models response: %v", err)
	}
	if len(body.Data) == 0 {
		t.Fatal("models response has no data")
	}
	for _, model := range body.Data {
		if model.ID == "" || model.Object != "model" || model.OwnedBy == "" {
			t.Fatalf("invalid model entry: %#v", model)
		}
	}
}

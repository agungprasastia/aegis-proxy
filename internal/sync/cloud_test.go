package sync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aegis-proxy/aegis/internal/config"
)

func TestCloudConfigRoundTrip(t *testing.T) {
	client, err := NewCloudClient("http://sync.local", "secret")
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.APIKey = "local-secret"
	cfg.DashboardPassword = "password"
	cfg.RTKEnabled = true
	cfg.SyncEndpoint = "http://sync.local"
	payload, err := client.ExportEncrypted("device-1", ExportConfig(cfg))
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if strings.Contains(string(payload.Data), "local-secret") || strings.Contains(string(payload.Data), "password") {
		t.Fatal("excluded secrets leaked into encrypted sync payload")
	}
	exported, err := client.ImportEncrypted(*payload)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if !exported.RTKEnabled || exported.SyncEndpoint != "http://sync.local" || exported.ProxyPort != 3130 {
		t.Fatalf("round trip mismatch: %+v", exported)
	}
}

func TestCloudSyncServerUploadDownloadMetaConflict(t *testing.T) {
	server := NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	client, err := NewCloudClient(httpServer.URL, "secret")
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	client.HTTP = httpServer.Client()
	payload, err := client.ExportEncrypted("device-1", ExportConfig(config.DefaultConfig()))
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if err := client.Upload(context.Background(), payload); err != nil {
		t.Fatalf("upload: %v", err)
	}
	downloaded, err := client.Download(context.Background())
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if downloaded.DeviceID != "device-1" {
		t.Fatalf("download mismatch: %+v", downloaded)
	}

	metaResp, err := http.Get(httpServer.URL + "/sync/meta")
	if err != nil {
		t.Fatalf("meta request: %v", err)
	}
	defer metaResp.Body.Close()
	var meta map[string]any
	if err := json.NewDecoder(metaResp.Body).Decode(&meta); err != nil {
		t.Fatalf("meta decode: %v", err)
	}
	if meta["exists"] != true {
		t.Fatalf("bad meta: %+v", meta)
	}

	older := *payload
	older.UpdatedAt = time.Now().UTC().Add(-time.Hour)
	body, _ := json.Marshal(older)
	req, _ := http.NewRequest(http.MethodPut, httpServer.URL+"/sync", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpServer.Client().Do(req)
	if err != nil {
		t.Fatalf("conflict request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want conflict", resp.StatusCode)
	}
}

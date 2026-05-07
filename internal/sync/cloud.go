package sync

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aegis-proxy/aegis/internal/config"
)

type CloudPayload struct {
	Version   int       `json:"version"`
	DeviceID  string    `json:"device_id"`
	UpdatedAt time.Time `json:"updated_at"`
	Data      string    `json:"data"`
}

type ExportedConfig struct {
	ProxyHost       string   `json:"proxy_host"`
	ProxyPort       int      `json:"proxy_port"`
	DashboardHost   string   `json:"dashboard_host"`
	DashboardPort   int      `json:"dashboard_port"`
	FilterMode      string   `json:"filter_mode"`
	FilterTemplates []string `json:"filter_templates"`
	RTKEnabled      bool     `json:"rtk_enabled"`
	SyncEndpoint    string   `json:"sync_endpoint,omitempty"`
}

type Client struct {
	Endpoint string
	Key      []byte
	HTTP     *http.Client
}

func NewCloudClient(endpoint, passphrase string) (*Client, error) {
	if passphrase == "" {
		return nil, errors.New("sync passphrase is required")
	}
	sum := sha256.Sum256([]byte(passphrase))
	return &Client{Endpoint: endpoint, Key: sum[:], HTTP: http.DefaultClient}, nil
}

func ExportConfig(cfg *config.Config) ExportedConfig {
	return ExportedConfig{ProxyHost: cfg.ProxyHost, ProxyPort: cfg.ProxyPort, DashboardHost: cfg.DashboardHost, DashboardPort: cfg.DashboardPort, FilterMode: cfg.FilterMode, FilterTemplates: append([]string(nil), cfg.FilterTemplates...), RTKEnabled: cfg.RTKEnabled, SyncEndpoint: cfg.SyncEndpoint}
}

func ApplyConfig(cfg *config.Config, exported ExportedConfig) {
	if exported.ProxyHost != "" {
		cfg.ProxyHost = exported.ProxyHost
	}
	if exported.ProxyPort != 0 {
		cfg.ProxyPort = exported.ProxyPort
	}
	if exported.DashboardHost != "" {
		cfg.DashboardHost = exported.DashboardHost
	}
	if exported.DashboardPort != 0 {
		cfg.DashboardPort = exported.DashboardPort
	}
	if exported.FilterMode != "" {
		cfg.FilterMode = exported.FilterMode
	}
	if exported.FilterTemplates != nil {
		cfg.FilterTemplates = append([]string(nil), exported.FilterTemplates...)
	}
	cfg.RTKEnabled = exported.RTKEnabled
	if exported.SyncEndpoint != "" {
		cfg.SyncEndpoint = exported.SyncEndpoint
	}
}

func (c *Client) ExportEncrypted(deviceID string, exported ExportedConfig) (*CloudPayload, error) {
	data, err := json.Marshal(exported)
	if err != nil {
		return nil, err
	}
	encrypted, err := c.encrypt(data)
	if err != nil {
		return nil, err
	}
	return &CloudPayload{Version: 1, DeviceID: deviceID, UpdatedAt: time.Now().UTC(), Data: encrypted}, nil
}

func (c *Client) ImportEncrypted(payload CloudPayload) (ExportedConfig, error) {
	plain, err := c.decrypt(payload.Data)
	if err != nil {
		return ExportedConfig{}, err
	}
	var exported ExportedConfig
	if err := json.Unmarshal(plain, &exported); err != nil {
		return ExportedConfig{}, err
	}
	return exported, nil
}

func (c *Client) Upload(ctx context.Context, payload *CloudPayload) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.Endpoint+"/sync", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("sync upload failed: status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Download(ctx context.Context) (*CloudPayload, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Endpoint+"/sync", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("sync download failed: status %d", resp.StatusCode)
	}
	var payload CloudPayload
	return &payload, json.NewDecoder(resp.Body).Decode(&payload)
}

func (c *Client) encrypt(plain []byte) (string, error) {
	block, err := aes.NewCipher(c.Key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, plain, nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (c *Client) decrypt(encoded string) ([]byte, error) {
	sealed, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(c.Key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("encrypted payload too short")
	}
	nonce, data := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	return gcm.Open(nil, nonce, data, nil)
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

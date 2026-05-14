package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── GetBackend ─────────────────────────────────────────────────────────────────

func TestGetBackend(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "b-cfg", DNSName: "b.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	backend := &types.Backend{ConfigID: cfg.ID, IP: "10.0.0.1", Weight: 1, Enabled: true}
	require.NoError(t, s.CreateBackend(context.Background(), backend))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/backends/"+backend.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	result := resp.Data.(map[string]interface{})
	assert.Equal(t, backend.ID, result["id"])
	assert.Equal(t, "10.0.0.1", result["ip"])
}

func TestGetBackend_NotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/backends/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── DeleteBackend error paths ──────────────────────────────────────────────────

func TestDeleteBackend_NotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/backends/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── AddBackend error paths ──────────────────────────────────────────────────────

func TestAddBackend_ConfigNotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	body := `{"ip": "1.2.3.4", "weight": 1, "enabled": true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/nonexistent/backends", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAddBackend_InvalidIP(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "ip-cfg", DNSName: "ip.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	body := `{"ip": "not-an-ip", "weight": 1, "enabled": true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/"+cfg.ID+"/backends", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── UpdateBackend error paths ─────────────────────────────────────────────────

func TestUpdateBackend_NotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	body := `{"ip": "1.2.3.4", "weight": 1}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/backends/nonexistent", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── ConfigureHealthCheck ────────────────────────────────────────────────────────

func TestConfigureHealthCheck_Update(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "hc-upd", DNSName: "hc-upd.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	// Create initial
	body := `{"type":"tcp","interval_seconds":10,"timeout_seconds":5,"threshold_healthy":2,"threshold_unhealthy":3}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/"+cfg.ID+"/health-check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Update it
	body2 := `{"type":"icmp","interval_seconds":30,"timeout_seconds":10,"threshold_healthy":3,"threshold_unhealthy":5}`
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/configs/"+cfg.ID+"/health-check", bytes.NewBufferString(body2))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	result := resp.Data.(map[string]interface{})
	assert.Equal(t, "icmp", result["type"])
}

func TestConfigureHealthCheck_ConfigNotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	body := `{"type":"tcp","interval_seconds":10,"timeout_seconds":5,"threshold_healthy":2,"threshold_unhealthy":3}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/nonexistent/health-check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteHealthCheck(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "hc-del", DNSName: "hc-del.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	hc := &types.HealthCheck{
		ConfigID: cfg.ID, Type: "tcp",
		IntervalSeconds: 10, TimeoutSeconds: 5,
		ThresholdHealthy: 2, ThresholdUnhealthy: 3,
	}
	require.NoError(t, s.CreateHealthCheck(context.Background(), hc))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/configs/"+cfg.ID+"/health-check", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestGetHealthCheck_NotFound(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "hc-nf", DNSName: "hc-nf.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID+"/health-check", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetBackendHealth_UnknownState(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "bh-cfg", DNSName: "bh.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	backend := &types.Backend{ConfigID: cfg.ID, IP: "10.1.1.1", Weight: 1, Enabled: true}
	require.NoError(t, s.CreateBackend(context.Background(), backend))

	// No health state created — should return unknown state
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/backends/"+backend.ID+"/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	result := resp.Data.(map[string]interface{})
	assert.Equal(t, "unknown", result["status"])
}

// ─── DNS Provider handlers ──────────────────────────────────────────────────────

func TestConfigureDNSProvider_Update(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "dns-upd", DNSName: "dns-upd.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	// Create initial
	body := `{"provider_type":"mock","config_json":"{\"zone\":\"z1\"}"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/"+cfg.ID+"/dns-provider", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Update it with a different provider type
	body2 := `{"provider_type":"cloudflare","config_json":"{\"api_token\":\"tok\",\"zone_id\":\"z\"}"}`
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/configs/"+cfg.ID+"/dns-provider", bytes.NewBufferString(body2))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
	result := resp.Data.(map[string]interface{})
	assert.Equal(t, "cloudflare", result["provider_type"])
}

func TestConfigureDNSProvider_ConfigNotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	body := `{"provider_type":"mock","config_json":"{}"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/nonexistent/dns-provider", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestConfigureDNSProvider_InvalidType(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "dns-inv", DNSName: "dns-inv.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	body := `{"provider_type":"route53","config_json":"{}"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/"+cfg.ID+"/dns-provider", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetDNSProvider_NotFound(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "dns-nf", DNSName: "dns-nf.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID+"/dns-provider", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteDNSProvider(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "dns-del", DNSName: "dns-del.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	provider := &types.DNSProviderConfig{
		ConfigID: cfg.ID, ProviderType: "mock", ConfigJSON: `{}`,
	}
	require.NoError(t, s.CreateDNSProvider(context.Background(), provider))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/configs/"+cfg.ID+"/dns-provider", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify deleted
	_, err := s.GetDNSProvider(context.Background(), cfg.ID)
	assert.Error(t, err)
}

func TestDeleteDNSProvider_ConfigNotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/configs/nonexistent/dns-provider", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── Config error paths ──────────────────────────────────────────────────────────

func TestUpdateConfig_NotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	body := `{"name":"x","dns_name":"x.example.com","dns_ttl":60,"lb_method":"round_robin"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/configs/nonexistent", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteConfig_NotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/configs/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── sameSiteFromString ───────────────────────────────────────────────────────

func TestSameSiteFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"Strict", 3},
		{"None", 4},
		{"Lax", 2},   // default
		{"", 2},      // default
		{"other", 2}, // default
	}
	for _, tt := range tests {
		got := sameSiteFromString(tt.input)
		assert.Equal(t, tt.expected, int(got), "input: %q", tt.input)
	}
}

// ─── GetConfigStatus ───────────────────────────────────────────────────────────

func TestGetConfigStatus_NoBackends(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{Name: "st-cfg", DNSName: "st.example.com", DNSTTL: 60, LBMethod: types.RoundRobin}
	require.NoError(t, s.CreateConfig(context.Background(), cfg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID+"/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	result := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(0), result["backend_count"])
}

// ─── ListBackends error path ────────────────────────────────────────────────────

func TestListBackends_ConfigNotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/nonexistent/backends", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── buildDetails ────────────────────────────────────────────────────────────────

func TestBuildDetails_Empty(t *testing.T) {
	result := buildDetails(nil)
	assert.Equal(t, "{}", result)
}

func TestBuildDetails_WithChanges(t *testing.T) {
	changes := []ChangeEntry{
		{Field: "name", Old: "old", New: "new"},
	}
	result := buildDetails(changes)
	assert.Contains(t, result, "name")
	assert.Contains(t, result, "old")
	assert.Contains(t, result, "new")
}

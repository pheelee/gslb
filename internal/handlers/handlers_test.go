package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pheelee/gslb/internal/db"
	"github.com/pheelee/gslb/internal/health"
	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	dbConn, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	err = db.Migrate(dbConn)
	require.NoError(t, err)

	return dbConn
}

func setupTestHandlers(t *testing.T) (*Handlers, store.Store, *sql.DB) {
	dbConn := setupTestDB(t)
	s := store.New(dbConn, nil)
	h := health.NewEngine(2)
	h.Start()
	t.Cleanup(func() { h.Stop() })
	return New(s, h), s, dbConn
}

func setupTestRouter(h *Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return NewRouter(h)
}

func TestCORS(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	// Dev origin is allowed — header should echo the request origin.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/api/v1/configs", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:5173", w.Header().Get("Access-Control-Allow-Origin"))

	// Same-origin request (no Origin header) — no CORS header needed.
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("OPTIONS", "/api/v1/configs", nil)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusNoContent, w2.Code)
	assert.Empty(t, w2.Header().Get("Access-Control-Allow-Origin"))
}

func TestHealthHandler(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp["status"])
}

func TestReadyHandler(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ready", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ready", resp["status"])
}

func TestGetSystemStatus(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	status, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "healthy", status["status"])
	assert.Equal(t, "1.0.0", status["version"])
}

func TestCreateConfig(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	body := `{
		"name": "Test Config",
		"dns_name": "test.example.com",
		"dns_ttl": 60,
		"lb_method": "round_robin"
	}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	cfg, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Test Config", cfg["name"])
	assert.Equal(t, "test.example.com", cfg["dns_name"])
	assert.NotEmpty(t, cfg["id"])
}

func TestCreateConfigValidationError(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	body := `{
		"name": "",
		"dns_name": "test.example.com",
		"dns_ttl": 60,
		"lb_method": "round_robin"
	}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Error)
}

func TestListConfigs(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	// Create test configs
	for i := 0; i < 3; i++ {
		cfg := &types.Config{
			Name:     "Test Config",
			DNSName:  "test" + string(rune('a'+i)) + ".example.com",
			DNSTTL:   60,
			LBMethod: types.RoundRobin,
		}
		err := s.CreateConfig(context.Background(), cfg)
		require.NoError(t, err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	configs, ok := resp.Data.([]interface{})
	require.True(t, ok)
	assert.Len(t, configs, 3)
}

func TestGetConfig(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	result, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, cfg.ID, result["id"])
}

func TestGetConfigNotFound(t *testing.T) {
	h, _, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateConfig(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	body := `{
		"name": "Updated Config",
		"dns_name": "updated.example.com",
		"dns_ttl": 120,
		"lb_method": "weighted"
	}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/configs/"+cfg.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	result, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Updated Config", result["name"])
	assert.Equal(t, "updated.example.com", result["dns_name"])
}

func TestDeleteConfig(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/configs/"+cfg.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify config is deleted
	_, err = s.GetConfig(context.Background(), cfg.ID)
	assert.Equal(t, store.ErrNotFound, err)
}

func TestAddBackend(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	body := `{
		"ip": "192.168.1.1",
		"port": 80,
		"weight": 5,
		"enabled": true
	}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/"+cfg.ID+"/backends", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	backend, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "192.168.1.1", backend["ip"])
	assert.Equal(t, float64(80), backend["port"])
	assert.Equal(t, cfg.ID, backend["config_id"])
}

func TestListBackends(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	// Create backends
	for i := 0; i < 3; i++ {
		backend := &types.Backend{
			ConfigID: cfg.ID,
			IP:       "192.168.1.1",
			Weight:   1,
			Enabled:  true,
		}
		err := s.CreateBackend(context.Background(), backend)
		require.NoError(t, err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID+"/backends", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	backends, ok := resp.Data.([]interface{})
	require.True(t, ok)
	assert.Len(t, backends, 3)
}

func TestUpdateBackend(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	backend := &types.Backend{
		ConfigID: cfg.ID,
		IP:       "192.168.1.1",
		Weight:   1,
		Enabled:  true,
	}
	err = s.CreateBackend(context.Background(), backend)
	require.NoError(t, err)

	body := `{
		"config_id": "` + cfg.ID + `",
		"ip": "192.168.1.2",
		"port": 8080,
		"weight": 10,
		"enabled": false
	}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/backends/"+backend.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	result, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "192.168.1.2", result["ip"])
	assert.Equal(t, float64(8080), result["port"])
}

func TestDeleteBackend(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	backend := &types.Backend{
		ConfigID: cfg.ID,
		IP:       "192.168.1.1",
		Weight:   1,
		Enabled:  true,
	}
	err = s.CreateBackend(context.Background(), backend)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/backends/"+backend.ID, nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify backend is deleted
	_, err = s.GetBackend(context.Background(), backend.ID)
	assert.Equal(t, store.ErrNotFound, err)
}

func TestConfigureHealthCheck(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	body := `{
		"type": "tcp",
		"interval_seconds": 30,
		"timeout_seconds": 10,
		"threshold_healthy": 2,
		"threshold_unhealthy": 3
	}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/"+cfg.ID+"/health-check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	hc, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "tcp", hc["type"])
}

func TestGetHealthCheck(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	hc := &types.HealthCheck{
		ConfigID:           cfg.ID,
		Type:               "tcp",
		IntervalSeconds:    10,
		TimeoutSeconds:     5,
		ThresholdHealthy:   2,
		ThresholdUnhealthy: 3,
	}
	err = s.CreateHealthCheck(context.Background(), hc)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID+"/health-check", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	result, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "tcp", result["type"])
}

func TestGetBackendHealth(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	backend := &types.Backend{
		ConfigID: cfg.ID,
		IP:       "192.168.1.1",
		Weight:   1,
		Enabled:  true,
	}
	err = s.CreateBackend(context.Background(), backend)
	require.NoError(t, err)

	// Create health state
	now := time.Now()
	state := &types.HealthState{
		BackendID:            backend.ID,
		Status:               types.StatusHealthy,
		ConsecutiveSuccesses: 5,
		LastCheckAt:          &now,
	}
	err = s.UpdateHealthState(context.Background(), state)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/backends/"+backend.ID+"/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	result, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "healthy", result["status"])
}

func TestConfigureDNSProvider(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	body := `{
		"provider_type": "mock",
		"config_json": "{\"zone_id\": \"test-zone\"}"
	}`

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/configs/"+cfg.ID+"/dns-provider", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	provider, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "mock", provider["provider_type"])
}

func TestGetDNSProvider(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	provider := &types.DNSProviderConfig{
		ConfigID:     cfg.ID,
		ProviderType: "route53",
		ConfigJSON:   `{"region": "us-east-1"}`,
	}
	err = s.CreateDNSProvider(context.Background(), provider)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID+"/dns-provider", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	result, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "route53", result["provider_type"])
}

func TestGetConfigStatus(t *testing.T) {
	h, s, _ := setupTestHandlers(t)
	router := setupTestRouter(h)

	cfg := &types.Config{
		Name:     "Test Config",
		DNSName:  "test.example.com",
		DNSTTL:   60,
		LBMethod: types.RoundRobin,
	}
	err := s.CreateConfig(context.Background(), cfg)
	require.NoError(t, err)

	// Create backends
	for i := 0; i < 2; i++ {
		backend := &types.Backend{
			ConfigID: cfg.ID,
			IP:       "192.168.1.1",
			Weight:   1,
			Enabled:  true,
		}
		err := s.CreateBackend(context.Background(), backend)
		require.NoError(t, err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/configs/"+cfg.ID+"/status", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	result, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, cfg.ID, result["config_id"])
	assert.Equal(t, "Test Config", result["name"])
	assert.Equal(t, float64(2), result["backend_count"])
}

func TestResponseStruct(t *testing.T) {
	resp := Response{
		Data:  map[string]string{"key": "value"},
		Error: "",
	}
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var parsed Response
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	dataMap, ok := parsed.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "value", dataMap["key"])
}

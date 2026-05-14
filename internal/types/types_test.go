package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthStatus_Constants(t *testing.T) {
	assert.Equal(t, HealthStatus("healthy"), StatusHealthy)
	assert.Equal(t, HealthStatus("unhealthy"), StatusUnhealthy)
	assert.Equal(t, HealthStatus("unknown"), StatusUnknown)
}

func TestLBMethod_Constants(t *testing.T) {
	assert.Equal(t, LBMethod("round_robin"), RoundRobin)
	assert.Equal(t, LBMethod("weighted"), Weighted)
}

func TestConfig_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	cfg := Config{
		ID:        "config-1",
		Name:      "test-config",
		DNSName:   "example.com",
		DNSTTL:    300,
		LBMethod:  RoundRobin,
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	var unmarshaled Config
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, cfg.ID, unmarshaled.ID)
	assert.Equal(t, cfg.Name, unmarshaled.Name)
	assert.Equal(t, cfg.DNSName, unmarshaled.DNSName)
	assert.Equal(t, cfg.DNSTTL, unmarshaled.DNSTTL)
	assert.Equal(t, cfg.LBMethod, unmarshaled.LBMethod)
}

func TestBackend_JSONSerialization(t *testing.T) {
	port := 8080
	be := Backend{
		ID:       "backend-1",
		ConfigID: "config-1",
		IP:       "192.168.1.10",
		Port:     &port,
		Weight:   10,
		Enabled:  true,
	}

	data, err := json.Marshal(be)
	require.NoError(t, err)

	var unmarshaled Backend
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, be.ID, unmarshaled.ID)
	assert.Equal(t, be.ConfigID, unmarshaled.ConfigID)
	assert.Equal(t, be.IP, unmarshaled.IP)
	assert.Equal(t, *be.Port, *unmarshaled.Port)
	assert.Equal(t, be.Weight, unmarshaled.Weight)
	assert.Equal(t, be.Enabled, unmarshaled.Enabled)
}

func TestBackend_PortOmitEmpty(t *testing.T) {
	beWithoutPort := Backend{
		ID:       "backend-1",
		ConfigID: "config-1",
		IP:       "192.168.1.10",
		Port:     nil,
		Weight:   10,
		Enabled:  true,
	}

	data, err := json.Marshal(beWithoutPort)
	require.NoError(t, err)

	assert.NotContains(t, string(data), `"port"`)
}

func TestHealthCheck_JSONSerialization(t *testing.T) {
	hc := HealthCheck{
		ID:                 "hc-1",
		ConfigID:           "config-1",
		Type:               "tcp",
		IntervalSeconds:    10,
		TimeoutSeconds:     5,
		ThresholdHealthy:   3,
		ThresholdUnhealthy: 3,
	}

	data, err := json.Marshal(hc)
	require.NoError(t, err)

	var unmarshaled HealthCheck
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, hc.ID, unmarshaled.ID)
	assert.Equal(t, hc.Type, unmarshaled.Type)
	assert.Equal(t, hc.IntervalSeconds, unmarshaled.IntervalSeconds)
}

func TestHealthState_JSONSerialization(t *testing.T) {
	now := time.Now().UTC()
	lastHealthy := time.Now().UTC().Add(-time.Minute)
	state := HealthState{
		BackendID:            "backend-1",
		Status:               StatusHealthy,
		ConsecutiveSuccesses: 5,
		ConsecutiveFailures:  0,
		LastCheckAt:          &now,
		LastHealthyAt:        &lastHealthy,
		LastError:            "",
	}

	data, err := json.Marshal(state)
	require.NoError(t, err)

	var unmarshaled HealthState
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, state.BackendID, unmarshaled.BackendID)
	assert.Equal(t, state.Status, unmarshaled.Status)
	assert.Equal(t, state.ConsecutiveSuccesses, unmarshaled.ConsecutiveSuccesses)
	assert.Equal(t, state.ConsecutiveFailures, unmarshaled.ConsecutiveFailures)
}

func TestHealthState_AllStatuses(t *testing.T) {
	statuses := []HealthStatus{StatusHealthy, StatusUnhealthy, StatusUnknown}

	for _, status := range statuses {
		state := HealthState{
			BackendID: "backend-1",
			Status:    status,
		}

		data, err := json.Marshal(state)
		require.NoError(t, err)

		var unmarshaled HealthState
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, status, unmarshaled.Status)
	}
}

func TestDNSProviderConfig_JSONSerialization(t *testing.T) {
	dpc := DNSProviderConfig{
		ID:           "dns-1",
		ConfigID:     "config-1",
		ProviderType: "route53",
		ConfigJSON:   `{"access_key":"xxx","secret_key":"yyy"}`,
	}

	data, err := json.Marshal(dpc)
	require.NoError(t, err)

	var unmarshaled DNSProviderConfig
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, dpc.ID, unmarshaled.ID)
	assert.Equal(t, dpc.ConfigID, unmarshaled.ConfigID)
	assert.Equal(t, dpc.ProviderType, unmarshaled.ProviderType)
	assert.Equal(t, dpc.ConfigJSON, unmarshaled.ConfigJSON)
}

func TestConfig_AllLBMethods(t *testing.T) {
	methods := []LBMethod{RoundRobin, Weighted}

	for _, method := range methods {
		cfg := Config{
			ID:       "config-1",
			Name:     "test",
			DNSName:  "example.com",
			DNSTTL:   300,
			LBMethod: method,
		}

		data, err := json.Marshal(cfg)
		require.NoError(t, err)

		var unmarshaled Config
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, method, unmarshaled.LBMethod)
	}
}

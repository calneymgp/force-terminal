package wcloud

import (
	"os"
	"testing"

	"github.com/wavetermdev/waveterm/pkg/wavebase"
)

func withDevMode(t *testing.T) {
	t.Helper()
	previousDev := wavebase.Dev_VarCache
	previousEndpoint := WCloudEndpoint_VarCache
	previousPing := WCloudPingEndpoint_VarCache
	wavebase.Dev_VarCache = "1"
	t.Cleanup(func() {
		wavebase.Dev_VarCache = previousDev
		WCloudEndpoint_VarCache = previousEndpoint
		WCloudPingEndpoint_VarCache = previousPing
	})
}

func TestDevBootstrapWithoutCloudOverridesUsesExistingDefaults(t *testing.T) {
	withDevMode(t)
	t.Setenv(WCloudEndpointVarName, "")
	t.Setenv(WCloudPingEndpointVarName, "")
	os.Unsetenv(WCloudEndpointVarName)
	os.Unsetenv(WCloudPingEndpointVarName)
	if err := CacheAndRemoveEnvVars(); err != nil {
		t.Fatalf("bootstrap failed without overrides: %v", err)
	}
	if got := GetEndpoint(); got != "https://api.waveterm.dev/central" {
		t.Fatalf("API endpoint: %s", got)
	}
	if got := GetPingEndpoint(); got != "https://ping.waveterm.dev/central" {
		t.Fatalf("ping endpoint: %s", got)
	}
}

func TestDevBootstrapRejectsInvalidExplicitCloudOverrides(t *testing.T) {
	withDevMode(t)
	for _, tc := range []struct{ api, ping string }{
		{api: "http://insecure.example", ping: ""},
		{api: "https://custom.example/api", ping: "http://insecure.example"},
		{api: "", ping: "https://custom.example/ping"},
	} {
		t.Setenv(WCloudEndpointVarName, tc.api)
		t.Setenv(WCloudPingEndpointVarName, tc.ping)
		if err := CacheAndRemoveEnvVars(); err == nil {
			t.Fatalf("accepted invalid overrides: api=%q ping=%q", tc.api, tc.ping)
		}
	}
}

func TestDevBootstrapKeepsValidExplicitCloudOverrides(t *testing.T) {
	withDevMode(t)
	t.Setenv(WCloudEndpointVarName, "https://custom.example/api")
	t.Setenv(WCloudPingEndpointVarName, "https://custom.example/ping")
	if err := CacheAndRemoveEnvVars(); err != nil {
		t.Fatalf("valid overrides failed: %v", err)
	}
	if got := GetEndpoint(); got != "https://custom.example/api" {
		t.Fatalf("API override: %s", got)
	}
	if got := GetPingEndpoint(); got != "https://custom.example/ping" {
		t.Fatalf("ping override: %s", got)
	}
}

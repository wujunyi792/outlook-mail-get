package main

import (
	"errors"
	"os"
	"testing"
)

func TestResolveConfigUsesEnvironmentDefaultsWithoutPersisting(t *testing.T) {
	withTempWorkingDir(t)
	t.Setenv(clientIDEnvName, "client-from-env")
	t.Setenv(tenantIDEnvName, "tenant-from-env")

	cfg, err := resolveFetchConfig(fetchOptions{limit: 3})
	if err != nil {
		t.Fatalf("resolveFetchConfig returned error: %v", err)
	}

	if cfg.clientID != "client-from-env" {
		t.Fatalf("expected client ID from env, got %q", cfg.clientID)
	}
	if cfg.tenantID != "tenant-from-env" {
		t.Fatalf("expected tenant ID from env, got %q", cfg.tenantID)
	}
	if cfg.email != "" {
		t.Fatalf("expected empty email selector, got %q", cfg.email)
	}

	configPath, err := configFilePath(false)
	if err != nil {
		t.Fatalf("configFilePath returned error: %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config file to remain absent, got err %v", err)
	}
}

func TestResolveConfigRequiresClientID(t *testing.T) {
	withTempWorkingDir(t)
	t.Setenv(clientIDEnvName, "")
	t.Setenv(tenantIDEnvName, "")

	if _, err := resolveFetchConfig(fetchOptions{limit: defaultLimit}); err == nil {
		t.Fatal("expected missing client ID error")
	}
}

func TestNormalizePersistedConfigMigratesLegacyAccount(t *testing.T) {
	cfg := normalizePersistedConfig(persistedConfig{
		ClientID:      "client",
		TenantID:      "consumers",
		Email:         "User@Example.com",
		HomeAccountID: "home-id",
	})

	if len(cfg.Accounts) != 1 {
		t.Fatalf("expected 1 migrated account, got %d", len(cfg.Accounts))
	}
	if cfg.Accounts[0].Email != "user@example.com" {
		t.Fatalf("unexpected migrated email: %q", cfg.Accounts[0].Email)
	}
	if cfg.DefaultEmail != "user@example.com" {
		t.Fatalf("unexpected default email: %q", cfg.DefaultEmail)
	}
	if cfg.Email != "" || cfg.HomeAccountID != "" {
		t.Fatalf("expected legacy fields to be cleared: %#v", cfg)
	}
}

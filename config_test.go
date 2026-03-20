package main

import (
	"errors"
	"os"
	"path/filepath"
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

func TestDataDirUsesEnvironmentOverride(t *testing.T) {
	override := filepath.Join(t.TempDir(), ".outlook-mail-get-test")
	t.Setenv(dataDirEnvName, override)

	dir, err := dataDir(true)
	if err != nil {
		t.Fatalf("dataDir returned error: %v", err)
	}
	if dir != override {
		t.Fatalf("expected override dir %q, got %q", override, dir)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("expected override dir to be created, got %v", err)
	}
}

func TestMigrateLegacyDataDirMovesProjectDataToUserDir(t *testing.T) {
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}

	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	projectDir := filepath.Join(tempDir, "project")
	legacyDir := filepath.Join(projectDir, legacyDataDirName)
	if err := os.MkdirAll(legacyDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, configFileName), []byte(`{"client_id":"legacy-client"}`), 0600); err != nil {
		t.Fatalf("WriteFile(config) error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, tokenCacheFileName), []byte("token-cache"), 0600); err != nil {
		t.Fatalf("WriteFile(token) error: %v", err)
	}

	t.Setenv("HOME", homeDir)
	t.Setenv(dataDirEnvName, "")
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := migrateLegacyDataDir(); err != nil {
		t.Fatalf("migrateLegacyDataDir returned error: %v", err)
	}

	targetDir := filepath.Join(homeDir, defaultDataDirName)
	if _, err := os.Stat(filepath.Join(targetDir, configFileName)); err != nil {
		t.Fatalf("expected migrated config file, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, tokenCacheFileName)); err != nil {
		t.Fatalf("expected migrated token cache, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, configFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected legacy config file to be removed, got %v", err)
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

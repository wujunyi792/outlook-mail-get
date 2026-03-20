package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
)

func TestRootHelpDoesNotCreateDataDir(t *testing.T) {
	withTempWorkingDir(t)

	stdout, stderr, err := executeCommand(t, "--help")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Fatalf("expected help output, got %q", stdout)
	}
	if !strings.Contains(stdout, "fetch") || !strings.Contains(stdout, "accounts") || !strings.Contains(stdout, "auth") {
		t.Fatalf("expected subcommands in help output, got %q", stdout)
	}
	if _, err := os.Stat("data"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected help to avoid creating data dir, got err %v", err)
	}
}

func TestAccountsListPrintsPersistedAccounts(t *testing.T) {
	withTempWorkingDir(t)

	err := storeConfigFile(persistedConfig{
		ClientID:     "client",
		TenantID:     "consumers",
		DefaultEmail: "default@example.com",
		Accounts: []persistedAccount{
			{Email: "default@example.com", HomeAccountID: "home-id"},
		},
	})
	if err != nil {
		t.Fatalf("storeConfigFile returned error: %v", err)
	}

	stdout, _, err := executeCommand(t, "accounts", "list")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if !containsLine(stdout, "default@example.com (default)") {
		t.Fatalf("expected account list output, got %q", stdout)
	}
}

func TestAuthResetClearsPersistedAccounts(t *testing.T) {
	withTempWorkingDir(t)

	err := storeConfigFile(persistedConfig{
		ClientID:     "client",
		TenantID:     "consumers",
		DefaultEmail: "default@example.com",
		Accounts: []persistedAccount{
			{Email: "default@example.com", HomeAccountID: "home-id"},
		},
	})
	if err != nil {
		t.Fatalf("storeConfigFile returned error: %v", err)
	}

	stdout, _, err := executeCommand(t, "auth", "reset")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if !containsLine(stdout, "已清除当前项目的本地认证状态。") {
		t.Fatalf("expected reset message, got %q", stdout)
	}

	cfg, err := loadConfigFile()
	if err != nil {
		t.Fatalf("loadConfigFile returned error: %v", err)
	}
	if cfg.ClientID != "client" || cfg.TenantID != "consumers" {
		t.Fatalf("expected client/tenant to be preserved, got %#v", cfg)
	}
	if cfg.DefaultEmail != "" || len(cfg.Accounts) != 0 {
		t.Fatalf("expected accounts to be cleared, got %#v", cfg)
	}
}

func TestRunMailCodeGetDoesNotPersistConfigWhenFetchFails(t *testing.T) {
	withTempWorkingDir(t)

	persistCalled := false
	err := runFetchCommand(context.Background(), io.Discard, fetchOptions{
		clientID: "client-id",
		tenantID: "consumers",
		limit:    defaultLimit,
	}, runtimeDeps{
		newTokenCredential: func(context.Context, config) (*graphAuth, error) {
			return &graphAuth{
				account: public.Account{
					PreferredUsername: "user@example.com",
					HomeAccountID:     "home-id",
				},
			}, nil
		},
		fetchRecentMessages: func(context.Context, tokenProvider, int) ([]messageInfo, []string, error) {
			return nil, nil, errors.New("fetch failed")
		},
		persistSuccessfulRun: func(config, public.Account) error {
			persistCalled = true
			return nil
		},
	})
	if err == nil {
		t.Fatal("expected fetch failure")
	}
	if persistCalled {
		t.Fatal("expected persistence to be skipped when fetch fails")
	}

	configPath, err := configFilePath(false)
	if err != nil {
		t.Fatalf("configFilePath returned error: %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config file to remain absent, got err %v", err)
	}
}

func TestFetchHelpShowsFetchFlags(t *testing.T) {
	withTempWorkingDir(t)

	stdout, stderr, err := executeCommand(t, "fetch", "--help")
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}
	if !strings.Contains(stdout, "--limit") || !strings.Contains(stdout, "--json") || !strings.Contains(stdout, "--reset-auth") {
		t.Fatalf("expected fetch flags in help output, got %q", stdout)
	}
}

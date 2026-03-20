package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultLimit       = 10
	defaultTenantID    = "consumers"
	clientIDEnvName    = "MAIL_CODE_GET_CLIENT_ID"
	tenantIDEnvName    = "MAIL_CODE_GET_TENANT_ID"
	dataDirEnvName     = "OUTLOOK_MAIL_GET_DATA_DIR"
	defaultDataDirName = ".outlook-mail-get"
	legacyDataDirName  = "data"
	configFileName     = "config.json"
	deviceLoginTimeout = 10 * time.Minute
)

type config struct {
	clientID  string
	tenantID  string
	email     string
	limit     int
	jsonOut   bool
	resetAuth bool
	persisted persistedConfig
}

type persistedConfig struct {
	ClientID     string             `json:"client_id"`
	TenantID     string             `json:"tenant_id"`
	DefaultEmail string             `json:"default_email,omitempty"`
	Accounts     []persistedAccount `json:"accounts,omitempty"`

	// legacy single-account fields, read for migration only
	Email         string `json:"email,omitempty"`
	HomeAccountID string `json:"home_account_id,omitempty"`
}

type persistedAccount struct {
	Email         string `json:"email"`
	HomeAccountID string `json:"home_account_id,omitempty"`
}

func tenantOrDefault(candidates ...string) string {
	for _, candidate := range candidates {
		if tenantID := strings.TrimSpace(candidate); tenantID != "" {
			return tenantID
		}
	}
	return defaultTenantID
}

func stringWithFallback(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(fallback)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func dataDir(create bool) (string, error) {
	if dir := strings.TrimSpace(os.Getenv(dataDirEnvName)); dir != "" {
		if create {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return "", fmt.Errorf("创建数据目录失败: %w", err)
			}
		}
		return dir, nil
	}

	dir, err := userDataDir()
	if err != nil {
		return "", err
	}
	if create {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", fmt.Errorf("创建数据目录失败: %w", err)
		}
	}

	return dir, nil
}

func userDataDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户 Home 目录失败: %w", err)
	}

	return filepath.Join(homeDir, defaultDataDirName), nil
}

func legacyProjectDataDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取当前项目目录失败: %w", err)
	}

	return filepath.Join(wd, legacyDataDirName), nil
}

func migrateLegacyDataDir() error {
	if strings.TrimSpace(os.Getenv(dataDirEnvName)) != "" {
		return nil
	}

	sourceDir, err := legacyProjectDataDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("读取旧数据目录失败: %w", err)
	}

	targetDir, err := dataDir(true)
	if err != nil {
		return err
	}

	movedAny := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		sourcePath := filepath.Join(sourceDir, entry.Name())
		targetPath := filepath.Join(targetDir, entry.Name())
		if _, err := os.Stat(targetPath); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("检查目标数据文件失败: %w", err)
		}

		if err := os.Rename(sourcePath, targetPath); err != nil {
			return fmt.Errorf("迁移数据文件 %q 失败: %w", entry.Name(), err)
		}
		movedAny = true
	}

	if movedAny {
		_ = os.Remove(sourceDir)
	}
	return nil
}

func configFilePath(create bool) (string, error) {
	dir, err := dataDir(create)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

func loadConfigFile() (persistedConfig, error) {
	path, err := configFilePath(false)
	if err != nil {
		return persistedConfig{}, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return persistedConfig{}, nil
		}
		return persistedConfig{}, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg persistedConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return persistedConfig{}, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return normalizePersistedConfig(cfg), nil
}

func storeConfigFile(cfg persistedConfig) error {
	path, err := configFilePath(true)
	if err != nil {
		return err
	}

	cfg = normalizePersistedConfig(cfg)
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置文件失败: %w", err)
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

func normalizePersistedConfig(cfg persistedConfig) persistedConfig {
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	cfg.TenantID = strings.TrimSpace(cfg.TenantID)
	cfg.DefaultEmail = normalizeEmail(cfg.DefaultEmail)

	var normalized []persistedAccount
	seen := map[string]struct{}{}

	appendAccount := func(email, homeAccountID string) {
		email = normalizeEmail(email)
		homeAccountID = strings.TrimSpace(homeAccountID)
		if email == "" {
			return
		}
		if _, ok := seen[email]; ok {
			return
		}

		seen[email] = struct{}{}
		normalized = append(normalized, persistedAccount{
			Email:         email,
			HomeAccountID: homeAccountID,
		})
	}

	for _, account := range cfg.Accounts {
		appendAccount(account.Email, account.HomeAccountID)
	}
	appendAccount(cfg.Email, cfg.HomeAccountID)

	cfg.Accounts = normalized
	if cfg.DefaultEmail == "" && len(cfg.Accounts) == 1 {
		cfg.DefaultEmail = cfg.Accounts[0].Email
	}

	// Clear legacy fields when writing back.
	cfg.Email = ""
	cfg.HomeAccountID = ""
	return cfg
}

func (cfg *persistedConfig) clearAccounts() {
	cfg.DefaultEmail = ""
	cfg.Accounts = nil
	cfg.Email = ""
	cfg.HomeAccountID = ""
}

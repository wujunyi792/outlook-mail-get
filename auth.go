package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	msalcache "github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
)

const tokenCacheFileName = "token-cache.bin"

var graphScopes = []string{
	"https://graph.microsoft.com/Mail.Read",
}

type tokenProvider interface {
	AccessToken(context.Context) (string, error)
}

type graphAuth struct {
	app      public.Client
	account  public.Account
	scopes   []string
	tenantID string
}

type fileTokenCache struct {
	path string
	mu   sync.Mutex
}

func newTokenCredential(ctx context.Context, cfg config) (*graphAuth, error) {
	cachePath, err := tokenCachePath(true)
	if err != nil {
		return nil, err
	}

	if cfg.resetAuth {
		if err := resetAuthState(cachePath, cfg.persisted); err != nil {
			return nil, err
		}
	}

	app, err := public.New(
		cfg.clientID,
		public.WithAuthority(authorityForTenant(cfg.tenantID)),
		public.WithCache(&fileTokenCache{path: cachePath}),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 MSAL 客户端失败: %w", err)
	}

	account, err := findCachedAccount(ctx, app, cfg)
	if err != nil {
		return nil, err
	}

	if account.IsZero() {
		account, err = authenticateWithDeviceCode(ctx, app, cfg)
		if err != nil {
			return nil, err
		}
	}

	return &graphAuth{
		app:      app,
		account:  account,
		scopes:   graphScopes,
		tenantID: cfg.tenantID,
	}, nil
}

func (g *graphAuth) AccessToken(ctx context.Context) (string, error) {
	result, err := g.app.AcquireTokenSilent(
		ctx,
		g.scopes,
		public.WithSilentAccount(g.account),
		public.WithTenantID(g.tenantID),
	)
	if err != nil {
		return "", fmt.Errorf("静默获取 token 失败，请尝试重新登录: %w", err)
	}
	return result.AccessToken, nil
}

func findCachedAccount(ctx context.Context, app public.Client, cfg config) (public.Account, error) {
	accounts, err := app.Accounts(ctx)
	if err != nil {
		return public.Account{}, fmt.Errorf("读取本地 token cache 失败: %w", err)
	}

	requestedEmail := normalizeEmail(cfg.email)

	var fallback public.Account
	for _, account := range accounts {
		if fallback.IsZero() {
			fallback = account
		}

		accountEmail := normalizeEmail(account.PreferredUsername)
		if target := cfg.persisted.accountByEmail(requestedEmail); target != nil && target.HomeAccountID != "" && account.HomeAccountID == target.HomeAccountID {
			return account, nil
		}
		if requestedEmail != "" && accountEmail == requestedEmail {
			return account, nil
		}
	}

	if requestedEmail == "" {
		if len(accounts) > 1 {
			return public.Account{}, errors.New("当前项目缓存了多个邮箱账号，请使用 --email 指定，或先运行 --list-accounts 查看可用账号")
		}
		return fallback, nil
	}
	return public.Account{}, nil
}

func authenticateWithDeviceCode(ctx context.Context, app public.Client, cfg config) (public.Account, error) {
	deviceCode, err := app.AcquireTokenByDeviceCode(ctx, graphScopes, public.WithTenantID(cfg.tenantID))
	if err != nil {
		return public.Account{}, fmt.Errorf("启动设备码登录失败: %w", err)
	}

	if strings.TrimSpace(deviceCode.Result.Message) != "" {
		fmt.Fprintln(os.Stderr, deviceCode.Result.Message)
	} else {
		fmt.Fprintf(
			os.Stderr,
			"请打开 %s 并输入代码 %s 完成登录。\n",
			deviceCode.Result.VerificationURL,
			deviceCode.Result.UserCode,
		)
	}

	result, err := deviceCode.AuthenticationResult(ctx)
	if err != nil {
		return public.Account{}, fmt.Errorf("设备码登录失败: %w", err)
	}

	actualEmail := normalizeEmail(result.Account.PreferredUsername)
	if cfg.email != "" && actualEmail != cfg.email {
		if err := app.RemoveAccount(ctx, result.Account); err != nil {
			return public.Account{}, fmt.Errorf(
				"登录的账号是 %s，不是请求的 %s；尝试撤回本次授权失败，错误账号可能仍保留在本地缓存中: %w",
				actualEmail,
				cfg.email,
				err,
			)
		}
		return public.Account{}, fmt.Errorf("登录的账号是 %s，不是请求的 %s；本次授权已撤回，请使用目标邮箱重新登录", actualEmail, cfg.email)
	}

	return result.Account, nil
}

func authorityForTenant(tenantID string) string {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		tenantID = defaultTenantID
	}
	return "https://login.microsoftonline.com/" + tenantID
}

func tokenCachePath(create bool) (string, error) {
	dir, err := dataDir(create)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, tokenCacheFileName), nil
}

func resetAuthState(cachePath string, persisted persistedConfig) error {
	if err := os.Remove(cachePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("删除 token cache 失败: %w", err)
	}

	persisted.clearAccounts()
	if err := storeConfigFile(persisted); err != nil {
		return err
	}
	return nil
}

func persistSuccessfulRun(cfg config, account public.Account) error {
	cfg.persisted.ClientID = cfg.clientID
	cfg.persisted.TenantID = cfg.tenantID

	if email := normalizeEmail(account.PreferredUsername); email != "" {
		cfg.persisted.upsertAccount(
			email,
			strings.TrimSpace(account.HomeAccountID),
		)
		cfg.persisted.DefaultEmail = email
	}
	cfg.persisted.Email = ""
	cfg.persisted.HomeAccountID = ""
	return storeConfigFile(cfg.persisted)
}

func (cfg persistedConfig) accountByEmail(email string) *persistedAccount {
	email = normalizeEmail(email)
	if email == "" {
		return nil
	}
	for _, account := range cfg.Accounts {
		if normalizeEmail(account.Email) == email {
			copy := account
			return &copy
		}
	}
	return nil
}

func (cfg *persistedConfig) upsertAccount(email, homeAccountID string) {
	email = normalizeEmail(email)
	homeAccountID = strings.TrimSpace(homeAccountID)
	if email == "" {
		return
	}

	for i := range cfg.Accounts {
		if normalizeEmail(cfg.Accounts[i].Email) != email {
			continue
		}
		cfg.Accounts[i].Email = email
		if homeAccountID != "" {
			cfg.Accounts[i].HomeAccountID = homeAccountID
		}
		return
	}

	cfg.Accounts = append(cfg.Accounts, persistedAccount{
		Email:         email,
		HomeAccountID: homeAccountID,
	})
}

func (f *fileTokenCache) Replace(ctx context.Context, unmarshaler msalcache.Unmarshaler, _ msalcache.ReplaceHints) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	data, err := os.ReadFile(f.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return unmarshaler.Unmarshal(data)
}

func (f *fileTokenCache) Export(ctx context.Context, marshaler msalcache.Marshaler, _ msalcache.ExportHints) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	data, err := marshaler.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(f.path, data, 0600)
}

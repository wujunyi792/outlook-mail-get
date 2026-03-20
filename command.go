package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type fetchOptions struct {
	clientID  string
	tenantID  string
	email     string
	limit     int
	jsonOut   bool
	resetAuth bool
}

type runtimeDeps struct {
	newTokenCredential   func(context.Context, config) (*graphAuth, error)
	fetchRecentMessages  func(context.Context, tokenProvider, int) ([]messageInfo, []string, error)
	persistSuccessfulRun func(config, public.Account) error
}

var defaultRuntimeDeps = runtimeDeps{
	newTokenCredential:   newTokenCredential,
	fetchRecentMessages:  fetchRecentMessages,
	persistSuccessfulRun: persistSuccessfulRun,
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "outlook-mail-get",
		Short:         "读取最近几封 Hotmail/Outlook 邮件",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newFetchCommand(defaultRuntimeDeps),
		newAccountsCommand(),
		newAuthCommand(),
	)

	return cmd
}

func newFetchCommand(deps runtimeDeps) *cobra.Command {
	opts := fetchOptions{
		limit: defaultLimit,
	}

	cmd := &cobra.Command{
		Use:     "fetch",
		Aliases: []string{"get", "read"},
		Short:   "抓取最近邮件",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFetchCommand(cmd.Context(), cmd.OutOrStdout(), opts, deps)
		},
	}

	addFetchFlags(cmd.Flags(), &opts)
	return cmd
}

func newAccountsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "查看当前项目已缓存的账号",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出当前项目已缓存的邮箱账号",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runListAccounts(cmd.OutOrStdout())
		},
	})

	return cmd
}

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "管理当前项目的认证状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "清除当前项目的本地认证状态",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResetAuth(cmd.OutOrStdout())
		},
	})

	return cmd
}

func addFetchFlags(flags *pflag.FlagSet, opts *fetchOptions) {
	flags.StringVar(&opts.clientID, "client-id", "", "Microsoft Entra 应用的 client ID")
	flags.StringVar(&opts.tenantID, "tenant-id", "", "Microsoft Entra tenant ID；Hotmail/Outlook.com 常用 consumers")
	flags.StringVar(&opts.email, "email", "", "指定要使用的邮箱账号")
	flags.IntVar(&opts.limit, "limit", defaultLimit, "返回最近多少条邮件")
	flags.BoolVar(&opts.jsonOut, "json", false, "以 JSON 输出")
	flags.BoolVar(&opts.resetAuth, "reset-auth", false, "删除本地认证记录并强制重新登录")
}

func runFetchCommand(ctx context.Context, out io.Writer, opts fetchOptions, deps runtimeDeps) error {
	if err := migrateLegacyDataDir(); err != nil {
		return err
	}

	cfg, err := resolveFetchConfig(opts)
	if err != nil {
		return err
	}

	runCtx, cancel := context.WithTimeout(ctx, deviceLoginTimeout)
	defer cancel()

	credential, err := deps.newTokenCredential(runCtx, cfg)
	if err != nil {
		return err
	}

	items, folders, err := deps.fetchRecentMessages(runCtx, credential, cfg.limit)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		if _, err := fmt.Fprintf(out, "未找到可读邮件。已检查文件夹: %s\n", strings.Join(folders, ", ")); err != nil {
			return err
		}
	} else if cfg.jsonOut {
		if err := printJSON(out, items); err != nil {
			return err
		}
	} else {
		if err := printTable(out, items); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "\n已检查文件夹: %s\n", strings.Join(folders, ", ")); err != nil {
			return err
		}
	}

	return deps.persistSuccessfulRun(cfg, credential.account)
}

func runListAccounts(out io.Writer) error {
	if err := migrateLegacyDataDir(); err != nil {
		return err
	}

	cfg, err := loadConfigFile()
	if err != nil {
		return err
	}
	return printAccounts(out, cfg)
}

func runResetAuth(out io.Writer) error {
	if err := migrateLegacyDataDir(); err != nil {
		return err
	}

	persisted, err := loadConfigFile()
	if err != nil {
		return err
	}

	if err := resetProjectAuthState(persisted); err != nil {
		return err
	}

	_, err = fmt.Fprintln(out, "已清除当前项目的本地认证状态。")
	return err
}

func resolveFetchConfig(opts fetchOptions) (config, error) {
	fileCfg, err := loadConfigFile()
	if err != nil {
		return config{}, err
	}

	if opts.limit <= 0 {
		return config{}, errors.New("--limit 必须大于 0")
	}

	envClientID := strings.TrimSpace(os.Getenv(clientIDEnvName))
	envTenantID := strings.TrimSpace(os.Getenv(tenantIDEnvName))

	cfg := config{
		clientID:  stringWithFallback(strings.TrimSpace(opts.clientID), stringWithFallback(envClientID, fileCfg.ClientID)),
		tenantID:  tenantOrDefault(strings.TrimSpace(opts.tenantID), envTenantID, fileCfg.TenantID),
		email:     normalizeEmail(stringWithFallback(strings.TrimSpace(opts.email), fileCfg.DefaultEmail)),
		limit:     opts.limit,
		jsonOut:   opts.jsonOut,
		resetAuth: opts.resetAuth,
		persisted: fileCfg,
	}

	if cfg.clientID == "" {
		return config{}, fmt.Errorf("缺少 client id。请传 --client-id，或设置环境变量 %s", clientIDEnvName)
	}

	return cfg, nil
}

func resetProjectAuthState(persisted persistedConfig) error {
	cachePath, err := tokenCachePath(false)
	if err != nil {
		return err
	}
	return resetAuthState(cachePath, persisted)
}

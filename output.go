package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

type messageInfo struct {
	Folder    string    `json:"folder"`
	ID        string    `json:"id"`
	Subject   string    `json:"subject"`
	From      string    `json:"from"`
	To        string    `json:"to,omitempty"`
	Date      time.Time `json:"date"`
	MessageID string    `json:"message_id,omitempty"`
	Unread    bool      `json:"unread"`
}

func printAccounts(out io.Writer, cfg persistedConfig) error {
	if len(cfg.Accounts) == 0 {
		_, err := fmt.Fprintln(out, "当前项目还没有缓存任何邮箱账号。")
		return err
	}

	for _, account := range cfg.Accounts {
		label := account.Email
		if account.Email == cfg.DefaultEmail {
			label += " (default)"
		}
		if _, err := fmt.Fprintln(out, label); err != nil {
			return err
		}
	}

	return nil
}

func printJSON(out io.Writer, items []messageInfo) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(items)
}

func printTable(out io.Writer, items []messageInfo) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "DATE\tFOLDER\tSTATUS\tFROM\tSUBJECT"); err != nil {
		return err
	}

	for _, item := range items {
		status := "read"
		if item.Unread {
			status = "unread"
		}

		if _, err := fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\n",
			sortTime(item).Format(time.DateTime),
			item.Folder,
			status,
			emptyFallback(item.From, "-"),
			item.Subject,
		); err != nil {
			return err
		}
	}

	return w.Flush()
}

func sortTime(msg messageInfo) time.Time {
	return msg.Date
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func finalizeMessages(items []messageInfo, limit int) []messageInfo {
	sort.Slice(items, func(i, j int) bool {
		return sortTime(items[i]).After(sortTime(items[j]))
	})
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func exitWithError(err error) {
	fmt.Fprintf(os.Stderr, "错误: %v\n", err)
	os.Exit(1)
}

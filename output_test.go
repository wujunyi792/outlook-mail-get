package main

import (
	"bytes"
	"testing"
	"time"
)

func TestPrintAccountsMarksDefault(t *testing.T) {
	cfg := persistedConfig{
		DefaultEmail: "default@example.com",
		Accounts: []persistedAccount{
			{Email: "default@example.com"},
			{Email: "other@example.com"},
		},
	}

	var output bytes.Buffer
	if err := printAccounts(&output, cfg); err != nil {
		t.Fatalf("printAccounts returned error: %v", err)
	}

	if !containsLine(output.String(), "default@example.com (default)") {
		t.Fatalf("expected default marker in output, got %q", output.String())
	}
	if !containsLine(output.String(), "other@example.com") {
		t.Fatalf("expected second account in output, got %q", output.String())
	}
}

func TestFinalizeMessagesSortsAndTrims(t *testing.T) {
	items := []messageInfo{
		{Subject: "old", Date: time.Date(2026, 3, 18, 8, 0, 0, 0, time.UTC)},
		{Subject: "new", Date: time.Date(2026, 3, 19, 8, 0, 0, 0, time.UTC)},
		{Subject: "mid", Date: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)},
	}

	got := finalizeMessages(items, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got))
	}
	if got[0].Subject != "new" || got[1].Subject != "mid" {
		t.Fatalf("unexpected order: %#v", got)
	}
}

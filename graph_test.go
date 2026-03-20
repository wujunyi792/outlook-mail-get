package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFormatRecipientList(t *testing.T) {
	got := formatRecipientList([]graphRecipient{
		{EmailAddress: graphEmailAddress{Name: "Alice", Address: "alice@example.com"}},
		{EmailAddress: graphEmailAddress{Address: "bob@example.com"}},
	})

	want := "Alice <alice@example.com>, bob@example.com"
	if got != want {
		t.Fatalf("formatRecipientList() = %q, want %q", got, want)
	}
}

func TestListFolderMessagesFollowsNextLink(t *testing.T) {
	oldBaseURL := graphBaseURL
	t.Cleanup(func() {
		graphBaseURL = oldBaseURL
	})

	requestCount := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/me/mailFolders/inbox/messages":
			_, _ = fmt.Fprintf(w, `{
				"value": [
					{"id":"1","subject":"first","receivedDateTime":"2026-03-20T08:00:00Z","internetMessageId":"m1","isRead":false,"from":{"emailAddress":{"name":"Alice","address":"alice@example.com"}},"toRecipients":[]},
					{"id":"2","subject":"second","receivedDateTime":"2026-03-20T07:59:00Z","internetMessageId":"m2","isRead":true,"from":{"emailAddress":{"name":"Bob","address":"bob@example.com"}},"toRecipients":[]}
				],
				"@odata.nextLink": "%s/page-2"
			}`, server.URL)
		case "/page-2":
			_, _ = io.WriteString(w, `{
				"value": [
					{"id":"3","subject":"third","receivedDateTime":"2026-03-20T07:58:00Z","internetMessageId":"m3","isRead":true,"from":{"emailAddress":{"name":"Carol","address":"carol@example.com"}},"toRecipients":[]}
				]
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	graphBaseURL = server.URL

	client := &graphClient{
		httpClient: server.Client(),
		credential: staticTokenProvider{token: "token"},
	}

	items, err := client.listFolderMessages(context.Background(), folderRef{
		WellKnownName: "inbox",
		DisplayName:   "Inbox",
	}, 3)
	if err != nil {
		t.Fatalf("listFolderMessages returned error: %v", err)
	}

	if requestCount != 2 {
		t.Fatalf("expected 2 paged requests, got %d", requestCount)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Subject != "first" || items[1].Subject != "second" || items[2].Subject != "third" {
		t.Fatalf("unexpected subjects: %#v", items)
	}
}

type staticTokenProvider struct {
	token string
}

func (p staticTokenProvider) AccessToken(context.Context) (string, error) {
	return p.token, nil
}

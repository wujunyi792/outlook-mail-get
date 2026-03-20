package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var graphBaseURL = "https://graph.microsoft.com/v1.0"

var defaultFolders = []folderRef{
	{WellKnownName: "inbox", DisplayName: "Inbox"},
	{WellKnownName: "junkemail", DisplayName: "Junk Email"},
	{WellKnownName: "deleteditems", DisplayName: "Deleted Items"},
}

type folderRef struct {
	WellKnownName string
	DisplayName   string
}

type graphClient struct {
	httpClient *http.Client
	credential tokenProvider
}

type graphMessageListResponse struct {
	Value    []graphMessage `json:"value"`
	NextLink string         `json:"@odata.nextLink"`
}

type graphMessage struct {
	ID                string           `json:"id"`
	Subject           string           `json:"subject"`
	ReceivedDateTime  time.Time        `json:"receivedDateTime"`
	InternetMessageID string           `json:"internetMessageId"`
	IsRead            bool             `json:"isRead"`
	From              graphRecipient   `json:"from"`
	ToRecipients      []graphRecipient `json:"toRecipients"`
}

type graphRecipient struct {
	EmailAddress graphEmailAddress `json:"emailAddress"`
}

type graphEmailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type graphErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func fetchRecentMessages(ctx context.Context, credential tokenProvider, limit int) ([]messageInfo, []string, error) {
	client := &graphClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		credential: credential,
	}

	perFolderLimit := limit
	if perFolderLimit < 5 {
		perFolderLimit = 5
	}

	collected := make([]messageInfo, 0, limit*len(defaultFolders))
	folders := make([]string, 0, len(defaultFolders))
	for _, folder := range defaultFolders {
		folders = append(folders, folder.DisplayName)

		items, err := client.listFolderMessages(ctx, folder, perFolderLimit)
		if err != nil {
			return nil, folders, fmt.Errorf("读取文件夹 %q 失败: %w", folder.DisplayName, err)
		}

		collected = append(collected, items...)
	}

	return finalizeMessages(collected, limit), folders, nil
}

func (c *graphClient) listFolderMessages(ctx context.Context, folder folderRef, limit int) ([]messageInfo, error) {
	values := url.Values{}
	values.Set("$top", strconv.Itoa(limit))
	values.Set("$orderby", "receivedDateTime desc")
	values.Set("$select", "id,subject,receivedDateTime,internetMessageId,isRead,from,toRecipients")

	endpoint := fmt.Sprintf("%s/me/mailFolders/%s/messages?%s", graphBaseURL, url.PathEscape(folder.WellKnownName), values.Encode())

	items := make([]messageInfo, 0, limit)
	for endpoint != "" && len(items) < limit {
		var response graphMessageListResponse
		if err := c.get(ctx, endpoint, &response); err != nil {
			return nil, err
		}

		for _, message := range response.Value {
			items = append(items, messageInfo{
				Folder:    folder.DisplayName,
				ID:        message.ID,
				Subject:   emptyFallback(message.Subject, "(无主题)"),
				From:      formatEmailAddress(message.From.EmailAddress),
				To:        formatRecipientList(message.ToRecipients),
				Date:      message.ReceivedDateTime,
				MessageID: message.InternetMessageID,
				Unread:    !message.IsRead,
			})
			if len(items) == limit {
				break
			}
		}

		endpoint = strings.TrimSpace(response.NextLink)
	}

	return items, nil
}

func (c *graphClient) get(ctx context.Context, endpoint string, out any) error {
	token, err := c.credential.AccessToken(ctx)
	if err != nil {
		return fmt.Errorf("获取 Graph access token 失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return parseGraphError(resp.StatusCode, body)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("解析 Graph 响应失败: %w", err)
	}

	return nil
}

func parseGraphError(statusCode int, body []byte) error {
	var envelope graphErrorEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil {
		code := strings.TrimSpace(envelope.Error.Code)
		message := strings.TrimSpace(envelope.Error.Message)
		if code != "" || message != "" {
			return fmt.Errorf("Graph API 返回 %d: %s %s", statusCode, code, message)
		}
	}

	return fmt.Errorf("Graph API 返回 %d: %s", statusCode, strings.TrimSpace(string(body)))
}

func formatRecipientList(recipients []graphRecipient) string {
	if len(recipients) == 0 {
		return ""
	}

	parts := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		formatted := formatEmailAddress(recipient.EmailAddress)
		if formatted == "" {
			continue
		}
		parts = append(parts, formatted)
	}

	return strings.Join(parts, ", ")
}

func formatEmailAddress(address graphEmailAddress) string {
	email := strings.TrimSpace(address.Address)
	name := strings.TrimSpace(address.Name)

	switch {
	case name != "" && email != "":
		return fmt.Sprintf("%s <%s>", name, email)
	case email != "":
		return email
	default:
		return name
	}
}

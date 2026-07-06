package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sms-relay/server/internal/applog"
)

var ErrNoAvatar = errors.New("bot has no avatar")

type Client struct {
	HTTP *http.Client
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 35 * time.Second}}
}

type BotInfo struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
}

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

func (c *Client) GetMe(ctx context.Context, token string) (*BotInfo, error) {
	var info BotInfo
	if err := c.call(ctx, token, "getMe", nil, &info); err != nil {
		return nil, err
	}
	if info.Username == "" {
		return nil, fmt.Errorf("bot has no username")
	}
	return &info, nil
}

func (c *Client) GetBotAvatar(ctx context.Context, token string) ([]byte, string, error) {
	bot, err := c.GetMe(ctx, token)
	if err != nil {
		return nil, "", err
	}

	vals := url.Values{}
	vals.Set("user_id", strconv.FormatInt(bot.ID, 10))
	vals.Set("limit", "1")

	var photos struct {
		TotalCount int `json:"total_count"`
		Photos     [][]struct {
			FileID string `json:"file_id"`
		} `json:"photos"`
	}
	if err := c.call(ctx, token, "getUserProfilePhotos", vals, &photos); err != nil {
		return nil, "", err
	}
	if photos.TotalCount == 0 || len(photos.Photos) == 0 || len(photos.Photos[0]) == 0 {
		return nil, "", ErrNoAvatar
	}

	sizes := photos.Photos[0]
	fileID := sizes[len(sizes)-1].FileID

	fileVals := url.Values{}
	fileVals.Set("file_id", fileID)
	var file struct {
		FilePath string `json:"file_path"`
	}
	if err := c.call(ctx, token, "getFile", fileVals, &file); err != nil {
		return nil, "", err
	}

	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", token, file.FilePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download avatar: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return data, contentType, nil
}

func (c *Client) PreparePolling(ctx context.Context, token string) error {
	// 仅删除 webhook，保留未读消息，避免用户已点的 /start 被丢弃
	return c.call(ctx, token, "deleteWebhook", nil, nil)
}

func (c *Client) LatestUpdateOffset(ctx context.Context, token string) (int, error) {
	updates, err := c.GetUpdates(ctx, token, 0, 0)
	if err != nil {
		return 0, err
	}
	offset := 0
	for _, upd := range updates {
		if upd.UpdateID >= offset {
			offset = upd.UpdateID + 1
		}
	}
	return offset, nil
}

func (c *Client) GetUpdates(ctx context.Context, token string, offset, timeoutSec int) ([]Update, error) {
	vals := url.Values{}
	if offset > 0 {
		vals.Set("offset", strconv.Itoa(offset))
	}
	if timeoutSec > 0 {
		vals.Set("timeout", strconv.Itoa(timeoutSec))
	}
	var updates []Update
	if err := c.call(ctx, token, "getUpdates", vals, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (c *Client) call(ctx context.Context, token, method string, query url.Values, out any) error {
	u := fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method)
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		applog.Error("telegram.client", "telegram HTTP request failed", "method", method, "error", err)
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var wrapped apiResponse
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return err
	}
	if !wrapped.OK {
		if wrapped.Description == "" {
			wrapped.Description = "telegram API error"
		}
		applog.Error("telegram.client", "telegram API call failed", "method", method, "description", wrapped.Description)
		return fmt.Errorf("%s", wrapped.Description)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(wrapped.Result, out)
}

func FormatChatID(id int64) string {
	return strconv.FormatInt(id, 10)
}

func MatchStartPayload(text, secret string) bool {
	payload := trimStartPayload(text)
	return payload == secret || payload == "link_"+secret
}

func ShouldBindMessage(text, secret, chatType string) bool {
	_ = chatType
	return MatchStartPayload(text, secret)
}

func trimStartPayload(text string) string {
	if len(text) < 6 || text[:6] != "/start" {
		return ""
	}
	payload := text[6:]
	for len(payload) > 0 && (payload[0] == ' ' || payload[0] == '\t') {
		payload = payload[1:]
	}
	if at := indexByte(payload, '@'); at >= 0 {
		payload = payload[:at]
	}
	for len(payload) > 0 && (payload[len(payload)-1] == ' ' || payload[len(payload)-1] == '\t') {
		payload = payload[:len(payload)-1]
	}
	return payload
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

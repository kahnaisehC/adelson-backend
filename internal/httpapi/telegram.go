package httpapi

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type telegramProvider struct {
	token   string
	baseURL string
	client  *http.Client
	log     *slog.Logger
}

func (s *Server) telegramWebhook(w http.ResponseWriter, r *http.Request) {
	p := &telegramProvider{
		token:   s.config.TelegramToken,
		baseURL: "https://api.telegram.org",
		client:  &http.Client{Timeout: 10 * time.Second},
		log:     s.log,
	}
	s.handleChatWebhook(p, w, r)
}

func (p *telegramProvider) Name() string {
	return "telegram"
}

func (p *telegramProvider) Verify(body []byte, r *http.Request) bool {
	if p.token == "" {
		return false
	}
	provided := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	return subtle.ConstantTimeCompare([]byte(provided), []byte(p.token)) == 1
}

type telegramUpdate struct {
	UpdateID int              `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	MessageID int           `json:"message_id"`
	From      *telegramUser `json:"from"`
	Chat      telegramChat  `json:"chat"`
	Text      string        `json:"text"`
	Date      int64         `json:"date"`
}

type telegramUser struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

func (p *telegramProvider) Parse(body []byte) ([]InboundMessage, error) {
	var update telegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		return nil, err
	}
	if update.Message == nil || strings.TrimSpace(update.Message.Text) == "" {
		return []InboundMessage{}, nil
	}

	msg := update.Message
	at := time.Now().UTC()
	if msg.Date != 0 {
		at = time.Unix(msg.Date, 0).UTC()
	}

	displayName := displayNameFromTelegramUser(msg.From)

	return []InboundMessage{
		{
			ExternalID:  strconv.FormatInt(msg.Chat.ID, 10),
			MessageID:   strconv.FormatInt(int64(update.UpdateID), 10),
			Text:        msg.Text,
			DisplayName: displayName,
			At:          at,
		},
	}, nil
}

func displayNameFromTelegramUser(u *telegramUser) string {
	if u == nil {
		return ""
	}
	fullName := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if fullName != "" {
		return fullName
	}
	return strings.TrimSpace(u.Username)
}

type telegramSendMessage struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func (p *telegramProvider) Reply(to, text string) error {
	if p.token == "" {
		return fmt.Errorf("telegram provider is not configured")
	}
	body, err := json.Marshal(telegramSendMessage{ChatID: to, Text: text})
	if err != nil {
		return fmt.Errorf("marshal telegram reply: %w", err)
	}

	url := p.baseURL + "/bot" + p.token + "/sendMessage"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram reply: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read telegram response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var tr telegramResponse
	if err := json.Unmarshal(respBody, &tr); err != nil {
		return fmt.Errorf("decode telegram response: %w", err)
	}
	if !tr.OK {
		if tr.Description != "" {
			return fmt.Errorf("telegram error: %s", tr.Description)
		}
		return fmt.Errorf("telegram returned ok=false")
	}
	return nil
}

// compile-time guard: telegramProvider implements ChatProvider.
var _ ChatProvider = (*telegramProvider)(nil)

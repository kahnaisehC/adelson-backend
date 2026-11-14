package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adelson-backend/internal/config"
	"adelson-backend/internal/store"
)

func telegramTestServer() http.Handler {
	return New(config.Config{
		JWTSecret:     "test-secret",
		ServiceToken:  "service-secret",
		TelegramToken: "tg-token",
	}, &store.Store{}, nil)
}

func TestTelegramVerify(t *testing.T) {
	p := &telegramProvider{token: "tg-token"}

	reqOK := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/telegram", nil)
	reqOK.Header.Set("X-Telegram-Bot-Api-Secret-Token", "tg-token")
	if !p.Verify(nil, reqOK) {
		t.Error("expected Verify to be true for matching secret")
	}

	reqWrong := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/telegram", nil)
	reqWrong.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong")
	if p.Verify(nil, reqWrong) {
		t.Error("expected Verify to be false for wrong secret")
	}

	reqMissing := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/telegram", nil)
	if p.Verify(nil, reqMissing) {
		t.Error("expected Verify to be false for missing secret")
	}

	empty := &telegramProvider{token: ""}
	if empty.Verify(nil, reqOK) {
		t.Error("expected Verify to be false when provider token is empty")
	}
}

func TestTelegramParse(t *testing.T) {
	p := &telegramProvider{}

	body := []byte(`{"update_id": 1001, "message": {"message_id": 7, "from": {"first_name":"María","last_name":"Gómez"}, "chat": {"id": 424242}, "text": "hola", "date": 1700000000}}`)
	msgs, err := p.Parse(body)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	m := msgs[0]
	if m.ExternalID != "424242" {
		t.Errorf("expected ExternalID 424242, got %q", m.ExternalID)
	}
	if m.MessageID != "1001" {
		t.Errorf("expected MessageID 1001, got %q", m.MessageID)
	}
	if m.Text != "hola" {
		t.Errorf("expected Text hola, got %q", m.Text)
	}
	if strings.TrimSpace(m.DisplayName) == "" {
		t.Error("expected non-empty DisplayName")
	}
	if !strings.Contains(m.DisplayName, "María") || !strings.Contains(m.DisplayName, "Gómez") {
		t.Errorf("expected DisplayName to contain first and last name, got %q", m.DisplayName)
	}

	empty, err := p.Parse([]byte(`{"update_id":1}`))
	if err != nil {
		t.Fatalf("unexpected parse error for empty update: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected 0 messages for update without message, got %d", len(empty))
	}
}

func TestTelegramWebhookRejectsInvalidSecret(t *testing.T) {
	body := strings.NewReader(`{"update_id":1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/telegram", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong")
	rec := httptest.NewRecorder()

	telegramTestServer().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "UNAUTHENTICATED") {
		t.Errorf("expected UNAUTHENTICATED error, got %s", rec.Body.String())
	}
}

func TestTelegramWebhookAcceptsEmptyUpdate(t *testing.T) {
	body := strings.NewReader(`{"update_id":1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/telegram", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "tg-token")
	rec := httptest.NewRecorder()

	telegramTestServer().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "received") {
		t.Errorf("expected received status, got %s", rec.Body.String())
	}
}

func TestTelegramReply(t *testing.T) {
	var gotChatID, gotText, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		_ = json.Unmarshal(body, &payload)
		gotChatID = payload["chat_id"]
		gotText = payload["text"]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	p := &telegramProvider{
		token:   "tg-token",
		baseURL: ts.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
		log:     slog.Default(),
	}
	if err := p.Reply("424242", "hola de vuelta"); err != nil {
		t.Fatalf("unexpected reply error: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/bottg-token/sendMessage") {
		t.Errorf("expected path /bottg-token/sendMessage, got %q", gotPath)
	}
	if gotChatID != "424242" {
		t.Errorf("expected chat_id 424242, got %q", gotChatID)
	}
	if gotText != "hola de vuelta" {
		t.Errorf("expected text hola de vuelta, got %q", gotText)
	}

	// ok=false returns an error
	errTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"description":"Forbidden: bot was blocked by the user"}`))
	}))
	defer errTS.Close()
	p2 := &telegramProvider{
		token:   "tg-token",
		baseURL: errTS.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
		log:     slog.Default(),
	}
	if err := p2.Reply("1", "text"); err == nil {
		t.Error("expected error when telegram returns ok=false")
	}

	// non-2xx returns an error
	errTS2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ok":false}`))
	}))
	defer errTS2.Close()
	p3 := &telegramProvider{
		token:   "tg-token",
		baseURL: errTS2.URL,
		client:  &http.Client{Timeout: 10 * time.Second},
		log:     slog.Default(),
	}
	if err := p3.Reply("1", "text"); err == nil {
		t.Error("expected error when telegram returns HTTP 500")
	}
}

func TestTelegramParseDisplayNameFallback(t *testing.T) {
	p := &telegramProvider{}

	withUsername := []byte(`{"update_id":2,"message":{"message_id":8,"from":{"username":"mariag"},"chat":{"id":99},"text":"ok","date":1700000000}}`)
	msgs, err := p.Parse(withUsername)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(msgs) != 1 || msgs[0].DisplayName != "mariag" {
		t.Errorf("expected username display name, got %q", msgs[0].DisplayName)
	}

	emptyUser := []byte(`{"update_id":3,"message":{"message_id":9,"from":{},"chat":{"id":99},"text":"ok","date":1700000000}}`)
	msgs, err = p.Parse(emptyUser)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(msgs) != 1 || msgs[0].DisplayName != "" {
		t.Errorf("expected empty display name, got %q", msgs[0].DisplayName)
	}
}

func TestTelegramParseNonTextMessage(t *testing.T) {
	p := &telegramProvider{}
	body := []byte(`{"update_id":4,"message":{"message_id":10,"from":{"first_name":"A"},"chat":{"id":1},"photo":[],"date":1700000000}}`)
	msgs, err := p.Parse(body)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages for non-text message, got %d", len(msgs))
	}
}

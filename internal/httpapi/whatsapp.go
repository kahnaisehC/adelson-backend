package httpapi

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type whatsappWebhookPayload struct {
	Object string                 `json:"object"`
	Entry  []whatsappWebhookEntry `json:"entry"`
}

type whatsappWebhookEntry struct {
	ID      string                  `json:"id"`
	Changes []whatsappWebhookChange `json:"changes"`
}

type whatsappWebhookChange struct {
	Field string               `json:"field"`
	Value whatsappWebhookValue `json:"value"`
}

type whatsappWebhookValue struct {
	MessagingProduct string                   `json:"messaging_product"`
	Metadata         whatsappWebhookMetadata  `json:"metadata"`
	Contacts         []whatsappWebhookContact `json:"contacts"`
	Messages         []whatsappWebhookMessage `json:"messages"`
	Statuses         []whatsappWebhookStatus  `json:"statuses"`
}

type whatsappWebhookMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type whatsappWebhookContact struct {
	WaID    string                        `json:"wa_id"`
	Profile whatsappWebhookContactProfile `json:"profile"`
}

type whatsappWebhookContactProfile struct {
	Name string `json:"name"`
}

type whatsappWebhookMessage struct {
	From      string `json:"from"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Text      struct {
		Body string `json:"body"`
	} `json:"text"`
}

type whatsappWebhookStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// whatsappProvider is the WhatsApp Business Cloud API (Meta) adapter for the
// shared chat pipeline.
type whatsappProvider struct {
	appSecret     string
	accessToken   string
	phoneNumberID string
	baseURL       string
	client        *http.Client
}

func (s *Server) whatsappWebhook(w http.ResponseWriter, r *http.Request) {
	if s.config.WhatsAppAppSecret == "" {
		writeError(w, r, http.StatusServiceUnavailable, "WEBHOOK_NOT_CONFIGURED", "El secreto del webhook no está configurado.", nil)
		return
	}
	version := s.config.WhatsAppAPIVersion
	if version == "" {
		version = "v25.0"
	}
	p := &whatsappProvider{
		appSecret:     s.config.WhatsAppAppSecret,
		accessToken:   s.config.WhatsAppToken,
		phoneNumberID: s.config.WhatsAppPhoneNumberID,
		baseURL:       "https://graph.facebook.com/" + version,
		client:        &http.Client{Timeout: 10 * time.Second},
	}
	s.handleChatWebhook(p, w, r)
}

func (p *whatsappProvider) Name() string { return "whatsapp" }

func (p *whatsappProvider) Verify(body []byte, r *http.Request) bool {
	if p.appSecret == "" {
		return false
	}
	return verifyHMACSHA256(p.appSecret, body, r.Header.Get("X-Hub-Signature-256"))
}

// Parse extracts inbound text messages. Status updates and non-text messages
// are ignored.
func (p *whatsappProvider) Parse(body []byte) ([]InboundMessage, error) {
	var payload whatsappWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	msgs := []InboundMessage{}
	if payload.Object != "whatsapp_business_account" {
		return msgs, nil
	}
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, m := range change.Value.Messages {
				if m.Type != "text" {
					continue
				}
				from := digitsOnly(strings.TrimSpace(m.From))
				if from == "" {
					continue
				}
				at := time.Now().UTC()
				if sec, err := strconv.ParseInt(m.Timestamp, 10, 64); err == nil {
					at = time.Unix(sec, 0).UTC()
				}
				first, last := namesFromContacts(change.Value.Contacts, m.From)
				msgs = append(msgs, InboundMessage{
					ExternalID:  from,
					MessageID:   m.ID,
					Text:        m.Text.Body,
					DisplayName: first + " " + last,
					At:          at,
				})
			}
		}
	}
	return msgs, nil
}

type whatsappSendMessage struct {
	MessagingProduct string           `json:"messaging_product"`
	RecipientType    string           `json:"recipient_type"`
	To               string           `json:"to"`
	Type             string           `json:"type"`
	Text             whatsappSendText `json:"text"`
}

type whatsappSendText struct {
	Body       string `json:"body"`
	PreviewURL bool   `json:"preview_url"`
}

// Reply sends a free-form text message. WhatsApp only allows this inside the
// 24h customer-service window opened by the customer's last inbound message,
// which always holds here because the bot only answers inbound messages.
func (p *whatsappProvider) Reply(to, text string) error {
	if p.accessToken == "" || p.phoneNumberID == "" {
		return fmt.Errorf("whatsapp provider is not configured")
	}
	body, err := json.Marshal(whatsappSendMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "text",
		Text:             whatsappSendText{Body: text},
	})
	if err != nil {
		return fmt.Errorf("marshal whatsapp reply: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, p.baseURL+"/"+p.phoneNumberID+"/messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build whatsapp request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.accessToken)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send whatsapp reply: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("whatsapp returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// compile-time guard: whatsappProvider implements ChatProvider.
var _ ChatProvider = (*whatsappProvider)(nil)

func (s *Server) verifyWhatsAppSignature(secret string, body []byte, header string) bool {
	return verifyHMACSHA256(secret, body, header)
}

func verifyHMACSHA256(secret string, body []byte, header string) bool {
	if !strings.HasPrefix(header, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	provided, err := hex.DecodeString(strings.TrimPrefix(header, "sha256="))
	if err != nil {
		return false
	}
	return hmac.Equal(provided, expected)
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func namesFromContacts(contacts []whatsappWebhookContact, from string) (string, string) {
	var name string
	for _, c := range contacts {
		if c.WaID == from {
			name = strings.TrimSpace(c.Profile.Name)
			break
		}
	}
	if name == "" && len(contacts) > 0 {
		name = strings.TrimSpace(contacts[0].Profile.Name)
	}
	if name == "" {
		return "Cliente", "WhatsApp"
	}
	parts := strings.Fields(name)
	if len(parts) == 1 {
		return parts[0], "WhatsApp"
	}
	return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
}

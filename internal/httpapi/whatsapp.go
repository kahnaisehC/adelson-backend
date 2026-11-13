package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"adelson-backend/internal/store"
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

func (s *Server) whatsappWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "No se pudo leer el cuerpo del webhook.", nil)
		return
	}

	if s.config.WhatsAppAppSecret == "" {
		writeError(w, r, http.StatusServiceUnavailable, "WEBHOOK_NOT_CONFIGURED", "El secreto del webhook no está configurado.", nil)
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")
	if !s.verifyWhatsAppSignature(s.config.WhatsAppAppSecret, body, signature) {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Firma del webhook inválida.", nil)
		return
	}

	var payload whatsappWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "El cuerpo JSON no es válido.", nil)
		return
	}

	if payload.Object != "whatsapp_business_account" {
		writeData(w, r, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field == "statuses" {
				s.log.Debug("skipping WhatsApp status update", slog.String("field", change.Field))
				continue
			}

			value := change.Value
			for _, m := range value.Messages {
				if m.Type != "text" {
					s.log.Debug("skipping non-text WhatsApp message", slog.String("type", m.Type), slog.String("message_id", m.ID))
					continue
				}

				from := strings.TrimSpace(m.From)
				phone := digitsOnly(from)
				if phone == "" {
					s.log.Debug("skipping WhatsApp message without phone", slog.String("message_id", m.ID))
					continue
				}

				ctx := r.Context()
				client, err := s.store.GetClientByPhone(ctx, phone)
				if errors.Is(err, store.ErrNotFound) {
					firstName, lastName := namesFromContacts(value.Contacts, from)
					dni := "WA-" + phone
					if len(dni) > 20 {
						dni = dni[:20]
					}
					email := "wa+" + phone + "@adelson.local"
					hash, _ := hashPassword("")
					client, err = s.store.CreateClient(ctx, store.ClientInput{
						FirstName:    firstName,
						LastName:     lastName,
						DNI:          dni,
						Email:        email,
						PasswordHash: hash,
					})
					if err != nil {
						s.databaseError(w, r, err)
						return
					}
					if _, err := s.store.CreatePhone(ctx, client.ID, "+"+phone, true); err != nil {
						s.databaseError(w, r, err)
						return
					}
				} else if err != nil {
					s.databaseError(w, r, err)
					return
				}

				session, err := s.store.GetOpenChatSession(ctx, client.ID)
				if err != nil {
					s.databaseError(w, r, err)
					return
				}
				if session == nil {
					created, err := s.store.CreateChatSession(ctx, client.ID)
					if err != nil {
						s.databaseError(w, r, err)
						return
					}
					session = &created
				}

				exists, err := s.store.MessageExists(ctx, session.ID, m.ID)
				if err != nil {
					s.databaseError(w, r, err)
					return
				}
				if exists {
					continue
				}

				fechaHora := time.Now().UTC().Format(time.RFC3339)
				if sec, err := strconv.ParseInt(m.Timestamp, 10, 64); err == nil {
					fechaHora = time.Unix(sec, 0).UTC().Format(time.RFC3339)
				}

				message := map[string]interface{}{
					"id":                randomID(),
					"remitenteTipo":     "cliente",
					"contenido":         m.Text.Body,
					"providerMessageId": m.ID,
					"estadoEntrega":     "recibido",
					"fechaHora":         fechaHora,
					"metadata": map[string]interface{}{
						"waFrom": from,
						"tipo":   m.Type,
					},
				}
				encoded, err := json.Marshal(message)
				if err != nil {
					s.databaseError(w, r, err)
					return
				}
				if _, err := s.store.AppendMessage(ctx, session.ID, encoded); err != nil {
					s.databaseError(w, r, err)
					return
				}
				if err := s.store.TouchClient(ctx, client.ID); err != nil {
					s.log.Debug("could not touch client", slog.String("error", err.Error()), slog.String("client_id", client.ID))
				}
			}
		}
	}

	writeData(w, r, http.StatusOK, map[string]string{"status": "received"})
}

func (s *Server) verifyWhatsAppSignature(secret string, body []byte, header string) bool {
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

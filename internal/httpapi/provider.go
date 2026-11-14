package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"adelson-backend/internal/store"
)

// ChatProvider abstracts an inbound messaging channel (Telegram, WhatsApp, etc.)
type ChatProvider interface {
	Name() string
	Verify(body []byte, r *http.Request) bool
	Parse(body []byte) ([]InboundMessage, error)
	Reply(to, text string) error
}

// InboundMessage represents a single normalized message coming from a channel.
type InboundMessage struct {
	ExternalID  string
	MessageID   string
	Text        string
	DisplayName string
	At          time.Time
}

func (s *Server) handleChatWebhook(provider ChatProvider, w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "No se pudo leer el cuerpo del webhook.", nil)
		return
	}

	if !provider.Verify(body, r) {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Firma del webhook inválida.", nil)
		return
	}

	msgs, err := provider.Parse(body)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "El cuerpo del webhook no es válido.", nil)
		return
	}

	ctx := r.Context()
	for _, msg := range msgs {
		if msg.ExternalID == "" || msg.MessageID == "" || msg.Text == "" {
			continue
		}

		email := fmt.Sprintf("%s+%s@adelson.local", provider.Name(), msg.ExternalID)
		dni := strings.ToUpper(provider.Name()) + "-" + msg.ExternalID
		if len(dni) > 20 {
			dni = dni[:20]
		}

		client, err := s.store.GetClientByEmail(ctx, email)
		if errors.Is(err, store.ErrNotFound) {
			firstName, lastName := splitDisplayName(msg.DisplayName)
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

		exists, err := s.store.MessageExists(ctx, session.ID, msg.MessageID)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		if exists {
			continue
		}

		fechaHora := msg.At.UTC().Format(time.RFC3339)
		if msg.At.IsZero() {
			fechaHora = time.Now().UTC().Format(time.RFC3339)
		}

		m := map[string]interface{}{
			"id":                randomID(),
			"remitenteTipo":     "cliente",
			"contenido":         msg.Text,
			"providerMessageId": msg.MessageID,
			"estadoEntrega":     "recibido",
			"fechaHora":         fechaHora,
			"metadata": map[string]interface{}{
				"channel":    provider.Name(),
				"externalId": msg.ExternalID,
			},
		}

		encoded, err := json.Marshal(m)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		if _, err := s.store.AppendMessage(ctx, session.ID, encoded); err != nil {
			s.databaseError(w, r, err)
			return
		}
		if err := s.store.TouchClient(ctx, client.ID); err != nil {
			s.log.Debug("could not touch client", "error", err.Error(), "client_id", client.ID)
		}

		if err := provider.Reply(msg.ExternalID, msg.Text); err != nil {
			s.log.Error("failed to reply to inbound message", "error", err.Error(), "channel", provider.Name(), "external_id", msg.ExternalID)
		}
	}

	writeData(w, r, http.StatusOK, map[string]string{"status": "received"})
}

// splitDisplayName splits a full name so the last whitespace token is the last
// name and everything before it is the first name. The fallback is
// "Cliente", "Telegram", and a single token becomes token, "Telegram".
func splitDisplayName(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Cliente", "Telegram"
	}
	parts := strings.Fields(name)
	if len(parts) == 1 {
		return parts[0], "Telegram"
	}
	return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
}

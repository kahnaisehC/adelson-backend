package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"adelson-backend/internal/config"
	"adelson-backend/internal/store"
)

func whatsappTestServer() http.Handler {
	return New(config.Config{
		JWTSecret:         "test-secret",
		ServiceToken:      "service-secret",
		WhatsAppAppSecret: "webhook-secret",
	}, &store.Store{}, nil)
}

func TestWhatsAppVerifyToken(t *testing.T) {
	t.Run("correct token echoes challenge", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/whatsapp?hub.verify_token=service-secret&hub.challenge=123", nil)
		response := httptest.NewRecorder()

		whatsappTestServer().ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", response.Code)
		}
		if body := strings.TrimSpace(response.Body.String()); body != "123" {
			t.Fatalf("expected challenge body '123', got %q", body)
		}
	})

	t.Run("wrong token returns forbidden", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/whatsapp?hub.verify_token=wrong&hub.challenge=123", nil)
		response := httptest.NewRecorder()

		whatsappTestServer().ServeHTTP(response, request)

		if response.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", response.Code)
		}
	})
}

func TestWhatsAppWebhookRejectsInvalidSignature(t *testing.T) {
	body := `{"object":"whatsapp_business_account","entry":[]}`

	t.Run("missing signature", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/whatsapp", strings.NewReader(body))
		response := httptest.NewRecorder()

		whatsappTestServer().ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", response.Code)
		}
	})

	t.Run("incorrect signature", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/whatsapp", strings.NewReader(body))
		request.Header.Set("X-Hub-Signature-256", "sha256=0000000000000000000000000000000000000000000000000000000000000000")
		response := httptest.NewRecorder()

		whatsappTestServer().ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", response.Code)
		}
	})
}

func TestWhatsAppWebhookAcceptsValidSignatureEmptyEntry(t *testing.T) {
	body := `{"object":"whatsapp_business_account","entry":[]}`
	sig := signWhatsAppBody(body, "webhook-secret")

	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/whatsapp", strings.NewReader(body))
	request.Header.Set("X-Hub-Signature-256", sig)
	response := httptest.NewRecorder()

	whatsappTestServer().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"status":"received"`) {
		t.Fatalf("expected received status, got %q", response.Body.String())
	}
}

func TestWhatsAppWebhookNotConfigured(t *testing.T) {
	server := New(config.Config{JWTSecret: "test-secret", ServiceToken: "service-secret"}, &store.Store{}, nil)
	body := `{"object":"whatsapp_business_account","entry":[]}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/whatsapp", strings.NewReader(body))
	request.Header.Set("X-Hub-Signature-256", signWhatsAppBody(body, "webhook-secret"))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", response.Code)
	}
}

func TestVerifyWhatsAppSignature(t *testing.T) {
	s := &Server{config: config.Config{WhatsAppAppSecret: "secret"}}
	body := []byte(`{"object":"whatsapp_business_account"}`)
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write(body)
	valid := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !s.verifyWhatsAppSignature("secret", body, valid) {
		t.Fatal("expected valid signature to pass")
	}
	if s.verifyWhatsAppSignature("wrong-secret", body, valid) {
		t.Fatal("expected wrong secret to fail")
	}
	if s.verifyWhatsAppSignature("secret", body, "invalid") {
		t.Fatal("expected missing sha256= prefix to fail")
	}
}

func TestDigitsOnly(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"+54 9 11 1234-5678", "5491112345678"},
		{"123", "123"},
		{"abc", ""},
		{"", ""},
	}

	for _, tc := range cases {
		if got := digitsOnly(tc.input); got != tc.expected {
			t.Errorf("digitsOnly(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func signWhatsAppBody(body, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

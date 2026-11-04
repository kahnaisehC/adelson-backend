package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"adelson-backend/internal/config"
	"adelson-backend/internal/store"
)

func testServer() http.Handler {
	return New(config.Config{JWTSecret: "test-secret", ServiceToken: "service-secret"}, &store.Store{}, nil)
}

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	testServer().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected request ID header")
	}
}

func TestReadyWithoutDatabase(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()

	testServer().ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", response.Code)
	}
}

func TestProtectedPropertyCreateRequiresAuthentication(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/inmuebles", nil)
	response := httptest.NewRecorder()

	testServer().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestRefreshRequiresRefreshCookie(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	response := httptest.NewRecorder()

	testServer().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestClientIPIgnoresSpoofedCloudflareHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	request.RemoteAddr = "192.0.2.10:4321"
	request.Header.Set("CF-Connecting-IP", "203.0.113.9")

	if got := clientIP(request); got != "192.0.2.10" {
		t.Fatalf("expected remote address, got %q", got)
	}

	request.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.1")
	if got := clientIP(request); got != "198.51.100.7" {
		t.Fatalf("expected forwarded address, got %q", got)
	}

	request.Header.Set("X-Forwarded-For", "not-an-ip")
	if got := clientIP(request); got != "192.0.2.10" {
		t.Fatalf("expected fallback to remote address, got %q", got)
	}
}

func TestAttemptLimiterBlocksAfterMax(t *testing.T) {
	limiter := newAttemptLimiter(2, time.Minute)
	const key = "account|admin@example.com"

	if !limiter.allow(key) {
		t.Fatal("expected first attempt to be allowed")
	}
	limiter.add(key)
	if !limiter.allow(key) {
		t.Fatal("expected second attempt to be allowed")
	}
	limiter.add(key)
	if limiter.allow(key) {
		t.Fatal("expected third attempt to be blocked")
	}
	if !limiter.allow("account|other@example.com") {
		t.Fatal("expected other keys to remain allowed")
	}
}

func TestAttemptLimiterWindowExpires(t *testing.T) {
	limiter := newAttemptLimiter(1, 20*time.Millisecond)
	const key = "account|admin@example.com"

	limiter.add(key)
	if limiter.allow(key) {
		t.Fatal("expected key to be blocked within window")
	}
	time.Sleep(30 * time.Millisecond)
	if !limiter.allow(key) {
		t.Fatal("expected key to be allowed after window")
	}
}

func TestPublicContactValidatesRequiredFields(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/contacto", strings.NewReader(`{"nombre":"Ana"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	testServer().ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", response.Code)
	}
}

func TestPublicInquiryRejectsUnknownProperty(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/inmuebles/not-a-uuid/consultas", strings.NewReader(`{"nombre":"Ana","apellido":"Pérez","correoElectronico":"ana@example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	testServer().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.Code)
	}
}

func TestGeneratedDNIHasExpectedFormat(t *testing.T) {
	first := generatedDNI()
	second := generatedDNI()

	if len(first) != 20 || !strings.HasPrefix(first, "WEB-") {
		t.Fatalf("unexpected DNI format: %q", first)
	}
	if first == second {
		t.Fatal("expected generated DNIs to differ")
	}
}

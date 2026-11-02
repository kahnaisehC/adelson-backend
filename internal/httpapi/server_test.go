package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

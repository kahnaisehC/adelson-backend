package geocoding

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestSearchUsesStructuredQueryAndCaches(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if got := r.URL.Query().Get("street"); got != "Av. San Martín 1450" {
			t.Errorf("unexpected street query: %q", got)
		}
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Error("expected user agent header")
		}
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{
				"lat":          "-27.4665",
				"lon":          "-58.9952",
				"display_name": "1450, Avenida San Martín, Resistencia, Argentina",
				"type":         "house",
			},
		})
	}))
	defer server.Close()

	client := New("test@example.com", time.Second)
	client.baseURL = server.URL
	client.minInterval = 0
	address := Address{Street: "Av. San Martín", Number: "1450", City: "Resistencia"}

	first, err := client.Search(context.Background(), address)
	if err != nil {
		t.Fatalf("first search: %v", err)
	}
	if len(first) != 1 || first[0].Latitude != -27.4665 || first[0].Longitude != -58.9952 {
		t.Fatalf("unexpected results: %+v", first)
	}

	if _, err := client.Search(context.Background(), address); err != nil {
		t.Fatalf("second search: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected cached results with 1 request, got %d", got)
	}
}

func TestSearchFallsBackToFreeForm(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if r.URL.Query().Get("street") != "" {
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}
		if got := r.URL.Query().Get("q"); got != "Calle Falsa 123, Villa Morra, Resistencia, Argentina" {
			t.Errorf("unexpected free-form query: %q", got)
		}
		_ = json.NewEncoder(w).Encode([]map[string]string{
			{
				"lat":          "-27.4514",
				"lon":          "-58.9867",
				"display_name": "Villa Morra, Resistencia, Argentina",
				"type":         "suburb",
			},
		})
	}))
	defer server.Close()

	client := New("", time.Second)
	client.baseURL = server.URL
	client.minInterval = 0
	address := Address{Street: "Calle Falsa", Number: "123", Neighborhood: "Villa Morra", City: "Resistencia"}

	results, err := client.Search(context.Background(), address)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].Type != "suburb" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("expected structured + fallback requests, got %d", got)
	}
}

func TestSearchRejectsIncompleteAddress(t *testing.T) {
	client := New("", time.Second)
	if _, err := client.Search(context.Background(), Address{City: "Resistencia"}); !errors.Is(err, ErrInvalidAddress) {
		t.Fatalf("expected ErrInvalidAddress, got %v", err)
	}
}

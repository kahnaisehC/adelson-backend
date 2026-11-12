package geocoding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL   = "https://nominatim.openstreetmap.org"
	defaultInterval  = time.Second
	defaultCacheTTL  = 24 * time.Hour
	defaultCacheSize = 500
)

var (
	ErrInvalidAddress = errors.New("invalid address")
	ErrUnavailable    = errors.New("geocoding provider unavailable")
)

type Address struct {
	Street       string
	Number       string
	Neighborhood string
	City         string
}

type Result struct {
	Latitude  float64
	Longitude float64
	Address   string
	Type      string
}

type cacheEntry struct {
	results []Result
	expires time.Time
}

type Client struct {
	baseURL     string
	contact     string
	http        *http.Client
	minInterval time.Duration
	cacheTTL    time.Duration
	cacheSize   int

	mu       sync.Mutex
	lastCall time.Time
	cache    map[string]cacheEntry
}

func New(contact string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Client{
		baseURL:     defaultBaseURL,
		contact:     strings.TrimSpace(contact),
		http:        &http.Client{Timeout: timeout},
		minInterval: defaultInterval,
		cacheTTL:    defaultCacheTTL,
		cacheSize:   defaultCacheSize,
		cache:       map[string]cacheEntry{},
	}
}

func (c *Client) Search(ctx context.Context, address Address) ([]Result, error) {
	street := joinParts(address.Street, address.Number)
	neighborhood := strings.TrimSpace(address.Neighborhood)
	city := strings.TrimSpace(address.City)
	if city == "" || (street == "" && neighborhood == "") {
		return nil, ErrInvalidAddress
	}

	key := strings.ToLower(strings.Join([]string{street, neighborhood, city}, "|"))
	if results, ok := c.fromCache(key); ok {
		return results, nil
	}

	var results []Result
	var err error
	if street != "" {
		if err = c.wait(ctx); err != nil {
			return nil, err
		}
		results, err = c.fetch(ctx, url.Values{
			"street":  {street},
			"city":    {city},
			"country": {"Argentina"},
		})
		if err != nil {
			return nil, err
		}
	}
	if len(results) == 0 {
		if err = c.wait(ctx); err != nil {
			return nil, err
		}
		results, err = c.fetch(ctx, url.Values{
			"q": {joinAddress(address)},
		})
		if err != nil {
			return nil, err
		}
	}

	c.storeCache(key, results)
	return results, nil
}

func (c *Client) fetch(ctx context.Context, values url.Values) ([]Result, error) {
	values.Set("format", "jsonv2")
	values.Set("limit", "5")
	values.Set("countrycodes", "ar")
	values.Set("accept-language", "es")

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/search?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", userAgent(c.contact))

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", ErrUnavailable, response.StatusCode)
	}

	var places []struct {
		Latitude  string `json:"lat"`
		Longitude string `json:"lon"`
		Address   string `json:"display_name"`
		Type      string `json:"type"`
	}
	if err := json.NewDecoder(response.Body).Decode(&places); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	results := make([]Result, 0, len(places))
	for _, place := range places {
		latitude, latErr := strconv.ParseFloat(place.Latitude, 64)
		longitude, lonErr := strconv.ParseFloat(place.Longitude, 64)
		if latErr != nil || lonErr != nil {
			continue
		}
		results = append(results, Result{
			Latitude:  latitude,
			Longitude: longitude,
			Address:   place.Address,
			Type:      place.Type,
		})
	}
	return results, nil
}

func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := time.Until(c.lastCall.Add(c.minInterval)); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.lastCall = time.Now()
	return nil
}

func (c *Client) fromCache(key string) ([]Result, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expires) {
		delete(c.cache, key)
		return nil, false
	}
	return entry.results, true
}

func (c *Client) storeCache(key string, results []Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= c.cacheSize {
		now := time.Now()
		for cachedKey, entry := range c.cache {
			if now.After(entry.expires) {
				delete(c.cache, cachedKey)
			}
		}
		if len(c.cache) >= c.cacheSize {
			c.cache = map[string]cacheEntry{}
		}
	}
	c.cache[key] = cacheEntry{results: results, expires: time.Now().Add(c.cacheTTL)}
}

func joinParts(parts ...string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, " ")
}

func joinAddress(address Address) string {
	parts := make([]string, 0, 4)
	if street := joinParts(address.Street, address.Number); street != "" {
		parts = append(parts, street)
	}
	if neighborhood := strings.TrimSpace(address.Neighborhood); neighborhood != "" {
		parts = append(parts, neighborhood)
	}
	if city := strings.TrimSpace(address.City); city != "" {
		parts = append(parts, city)
	}
	parts = append(parts, "Argentina")
	return strings.Join(parts, ", ")
}

func userAgent(contact string) string {
	if contact == "" {
		return "AdelsonInmobiliaria/1.0"
	}
	return "AdelsonInmobiliaria/1.0 (" + contact + ")"
}

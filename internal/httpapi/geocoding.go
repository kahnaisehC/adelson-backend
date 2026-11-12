package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"adelson-backend/internal/geocoding"
)

type addressGeocoder interface {
	Search(ctx context.Context, address geocoding.Address) ([]geocoding.Result, error)
}

type geocodeResult struct {
	Latitude  float64 `json:"latitud"`
	Longitude float64 `json:"longitud"`
	Address   string  `json:"direccion"`
	Type      string  `json:"tipo"`
}

func (s *Server) geocode(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.require(w, r, "Agente", "Administrador"); !ok {
		return
	}

	var input struct {
		Street       string `json:"calle"`
		Number       string `json:"numero"`
		Neighborhood string `json:"barrio"`
		City         string `json:"ciudad"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}

	address, err := validateGeocodeInput(input.Street, input.Number, input.Neighborhood, input.City)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if s.geocoder == nil {
		writeError(w, r, http.StatusServiceUnavailable, "GEOCODING_UNAVAILABLE", "El servicio de geocodificación no está disponible.", nil)
		return
	}

	timeout := s.config.GeocoderTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	results, err := s.geocoder.Search(ctx, address)
	if err != nil {
		if errors.Is(err, geocoding.ErrInvalidAddress) {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "La dirección no es válida.", nil)
			return
		}
		s.log.Error("geocode address", "error", err)
		status := http.StatusBadGateway
		code := "GEOCODING_FAILED"
		message := "No se pudo buscar la dirección."
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
			code = "GEOCODING_TIMEOUT"
			message = "La búsqueda de la dirección tardó demasiado."
		}
		writeError(w, r, status, code, message, nil)
		return
	}

	data := make([]geocodeResult, 0, len(results))
	for _, result := range results {
		data = append(data, geocodeResult{
			Latitude:  result.Latitude,
			Longitude: result.Longitude,
			Address:   result.Address,
			Type:      result.Type,
		})
	}
	writeData(w, r, http.StatusOK, data)
}

func validateGeocodeInput(street, number, neighborhood, city string) (geocoding.Address, error) {
	address := geocoding.Address{
		Street:       strings.TrimSpace(street),
		Number:       strings.TrimSpace(number),
		Neighborhood: strings.TrimSpace(neighborhood),
		City:         strings.TrimSpace(city),
	}
	if address.City == "" {
		return geocoding.Address{}, errors.New("la ciudad es obligatoria")
	}
	if address.Street == "" && address.Neighborhood == "" {
		return geocoding.Address{}, errors.New("indicá al menos la calle o el barrio")
	}
	if utf8.RuneCountInString(address.Street) > 120 ||
		utf8.RuneCountInString(address.Number) > 20 ||
		utf8.RuneCountInString(address.Neighborhood) > 100 ||
		utf8.RuneCountInString(address.City) > 100 {
		return geocoding.Address{}, errors.New("la dirección es demasiado larga")
	}
	return address, nil
}

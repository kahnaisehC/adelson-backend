package httpapi

import (
	"strings"
	"testing"
)

func TestValidateGeocodeInput(t *testing.T) {
	tests := []struct {
		name         string
		street       string
		number       string
		neighborhood string
		city         string
		wantErr      bool
	}{
		{name: "empty", wantErr: true},
		{name: "city only", city: "Resistencia", wantErr: true},
		{name: "street and city", street: "Av. San Martín", number: "1450", city: "Resistencia"},
		{name: "neighborhood and city", neighborhood: "Villa Morra", city: "Resistencia"},
		{name: "street too long", street: strings.Repeat("a", 121), city: "Resistencia", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address, err := validateGeocodeInput(test.street, test.number, test.neighborhood, test.city)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected validation error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if address.City != test.city {
				t.Fatalf("unexpected address: %+v", address)
			}
		})
	}
}

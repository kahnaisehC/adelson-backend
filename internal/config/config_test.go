package config

import "testing"

func TestProductionWorkerOriginIsRequired(t *testing.T) {
	origins := splitList("http://localhost:5173")
	if containsOrigin(origins, productionWorkerOrigin) {
		t.Fatal("production origin should not be present yet")
	}

	if !containsOrigin(append(origins, productionWorkerOrigin), productionWorkerOrigin) {
		t.Fatal("production origin should be present after adding it")
	}
}

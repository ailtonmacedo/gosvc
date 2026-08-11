package preset

import "testing"

func TestResolveReturnsIndependentFeatureSlice(t *testing.T) {
	t.Parallel()

	first, err := Resolve("minimal-api")
	if err != nil {
		t.Fatal(err)
	}
	first.Features[0] = "changed"

	second, err := Resolve("minimal-api")
	if err != nil {
		t.Fatal(err)
	}
	if second.Features[0] != "base" {
		t.Fatalf("Resolve() leaked mutable state: %v", second.Features)
	}
}

func TestResolveRejectsUnknownPreset(t *testing.T) {
	t.Parallel()

	if _, err := Resolve("missing"); err == nil {
		t.Fatal("Resolve() error = nil, want error")
	}
}

func TestPostgresPresetIncludesOperationalFeatures(t *testing.T) {
	t.Parallel()

	definition, err := Resolve("postgres-api")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"chi", "postgres", "migrations", "sqlc", "docker-compose"} {
		found := false
		for _, feature := range definition.Features {
			if feature == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("feature %q missing from %v", expected, definition.Features)
		}
	}
}

func TestEventDrivenPresetIncludesDistributedFeatures(t *testing.T) {
	t.Parallel()
	definition, err := Resolve("event-driven-api")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"redis", "kafka", "outbox", "idempotency", "dead-letter", "kubernetes"} {
		found := false
		for _, feature := range definition.Features {
			if feature == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("feature %q missing from %v", expected, definition.Features)
		}
	}
}

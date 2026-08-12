package cli

import "testing"

func TestParseNewOptionsAcceptsFlagsAfterProjectName(t *testing.T) {
	t.Parallel()

	options, err := parseNewOptions([]string{
		"order-service",
		"--module", "github.com/acme/order-service",
		"--preset", "postgres-api",
		"--dry-run",
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.Name != "order-service" || options.Module != "github.com/acme/order-service" || options.Preset != "postgres-api" || !options.DryRun {
		t.Fatalf("parseNewOptions() = %+v", options)
	}
}

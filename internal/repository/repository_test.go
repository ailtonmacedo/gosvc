package repository

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()
	value, err := Parse("https://github.com/acme/gosvc.git")
	if err != nil {
		t.Fatal(err)
	}
	if value.Slug() != "acme/gosvc" || value.Module() != "github.com/acme/gosvc" {
		t.Fatalf("unexpected repository: %+v", value)
	}
}

func TestParseRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "owner", "github.com/a/b/c", "owner/ bad"} {
		if _, err := Parse(value); err == nil {
			t.Fatalf("expected %q to fail", value)
		}
	}
}

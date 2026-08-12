package cli

import "testing"

func TestParseAddResourceOptions(t *testing.T) {
	t.Parallel()
	options, err := parseAddResourceOptions([]string{"--project", "./service", "product", "--crud", "--fields", "id:uuid,name:string"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Name != "product" || options.ProjectDir != "./service" || !options.CRUD {
		t.Fatalf("options = %+v", options)
	}
}

func TestParseAddResourceOptionsRequiresCRUD(t *testing.T) {
	t.Parallel()
	if _, err := parseAddResourceOptions([]string{"product", "--fields", "id:uuid"}); err == nil {
		t.Fatal("expected --crud error")
	}
}

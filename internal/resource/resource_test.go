package resource

import (
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	definition, err := Parse("product", "id:uuid,name:string,price:decimal,active:bool")
	if err != nil {
		t.Fatal(err)
	}
	if definition.Plural != "products" {
		t.Fatalf("plural = %q", definition.Plural)
	}
	if definition.Fields[2].Type != "int64" {
		t.Fatalf("decimal normalized to %q", definition.Fields[2].Type)
	}
}

func TestParseRejectsDuplicateAndMissingID(t *testing.T) {
	if _, err := Parse("product", "name:string,name:string"); err == nil {
		t.Fatal("expected duplicate error")
	}
	if _, err := Parse("product", "name:string"); err == nil {
		t.Fatal("expected missing id error")
	}
}

func TestParseRejectsStringID(t *testing.T) {
	if _, err := Parse("product", "id:string,name:string"); err == nil {
		t.Fatal("expected string id error")
	}
}

func TestAddAssignsStableMigrationNumber(t *testing.T) {
	resources := []Definition{DefaultOrder()}
	product, err := Parse("product", "id:uuid,name:string")
	if err != nil {
		t.Fatal(err)
	}
	resources, _, err = Add(resources, product)
	if err != nil {
		t.Fatal(err)
	}
	account, err := Parse("account", "id:int64,name:string")
	if err != nil {
		t.Fatal(err)
	}
	resources, _, err = Add(resources, account)
	if err != nil {
		t.Fatal(err)
	}
	versions := map[string]int{}
	for _, definition := range resources {
		versions[definition.Name] = definition.Migration
	}
	if versions["order"] != 1 || versions["product"] != 2 || versions["account"] != 3 {
		t.Fatalf("migration versions = %#v", versions)
	}
}

func TestLoadRejectsNegativeMigration(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/.gosvc"
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"schema_version":1,"resources":[{"name":"order","plural":"orders","migration":-1,"fields":[{"name":"id","type":"int64"}]}]}`)
	if err := os.WriteFile(path+"/resources.json", data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected invalid migration error")
	}
}

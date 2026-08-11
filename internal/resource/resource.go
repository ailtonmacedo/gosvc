package resource

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const RegistryPath = ".gosvc/resources.json"

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`)

type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Definition struct {
	Name      string  `json:"name"`
	Plural    string  `json:"plural"`
	Migration int     `json:"migration"`
	Fields    []Field `json:"fields"`
}

type Registry struct {
	SchemaVersion int          `json:"schema_version"`
	Resources     []Definition `json:"resources"`
}

func DefaultOrder() Definition {
	return Definition{Name: "order", Plural: "orders", Migration: 1, Fields: []Field{{Name: "id", Type: "int64"}, {Name: "status", Type: "string"}, {Name: "total_cents", Type: "int64"}}}
}

func Parse(name, fields string) (Definition, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if !identifierPattern.MatchString(name) {
		return Definition{}, fmt.Errorf("resource name %q must use lowercase snake_case", name)
	}
	if strings.TrimSpace(fields) == "" {
		return Definition{}, fmt.Errorf("at least one field is required")
	}
	definition := Definition{Name: name, Plural: pluralize(name)}
	seen := map[string]bool{}
	for _, token := range strings.Split(fields, ",") {
		parts := strings.Split(strings.TrimSpace(token), ":")
		if len(parts) != 2 {
			return Definition{}, fmt.Errorf("invalid field %q; expected name:type", token)
		}
		field := Field{Name: strings.TrimSpace(strings.ToLower(parts[0])), Type: strings.TrimSpace(strings.ToLower(parts[1]))}
		if !identifierPattern.MatchString(field.Name) {
			return Definition{}, fmt.Errorf("field name %q must use lowercase snake_case", field.Name)
		}
		if seen[field.Name] {
			return Definition{}, fmt.Errorf("duplicate field %q", field.Name)
		}
		seen[field.Name] = true
		if !supportedType(field.Type) {
			return Definition{}, fmt.Errorf("unsupported type %q for field %q; allowed: uuid, string, int64, integer, decimal, bool, datetime", field.Type, field.Name)
		}
		if field.Type == "integer" || field.Type == "decimal" {
			field.Type = "int64"
		}
		definition.Fields = append(definition.Fields, field)
	}
	id, ok := definition.Field("id")
	if !ok {
		return Definition{}, fmt.Errorf("resource must declare an id field")
	}
	if id.Type != "int64" && id.Type != "uuid" {
		return Definition{}, fmt.Errorf("id field must use int64 or uuid")
	}
	return definition, nil
}

func (d Definition) Validate() error {
	parsed, err := Parse(d.Name, fieldsString(d.Fields))
	if err != nil {
		return err
	}
	if d.Plural == "" {
		d.Plural = parsed.Plural
	}
	return nil
}

func (d Definition) Field(name string) (Field, bool) {
	for _, field := range d.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return Field{}, false
}

func (d Definition) ID() Field { field, _ := d.Field("id"); return field }

func Load(projectDir string) ([]Definition, error) {
	data, err := os.ReadFile(filepath.Join(projectDir, filepath.FromSlash(RegistryPath)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read resource registry: %w", err)
	}
	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("decode resource registry: %w", err)
	}
	if registry.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported resource registry schema version %d", registry.SchemaVersion)
	}
	maxMigration := 0
	for _, definition := range registry.Resources {
		if definition.Migration > maxMigration {
			maxMigration = definition.Migration
		}
	}
	usedMigrations := make(map[int]string, len(registry.Resources))
	for index := range registry.Resources {
		definition := &registry.Resources[index]
		if definition.Plural == "" {
			definition.Plural = pluralize(definition.Name)
		}
		if definition.Migration == 0 {
			maxMigration++
			definition.Migration = maxMigration
		}
		if definition.Migration < 1 {
			return nil, fmt.Errorf("resource %q uses invalid migration %d", definition.Name, definition.Migration)
		}
		if current, exists := usedMigrations[definition.Migration]; exists {
			return nil, fmt.Errorf("resources %q and %q use migration %d", current, definition.Name, definition.Migration)
		}
		usedMigrations[definition.Migration] = definition.Name
		if err := definition.Validate(); err != nil {
			return nil, fmt.Errorf("resource %q: %w", definition.Name, err)
		}
	}
	sort.Slice(registry.Resources, func(i, j int) bool { return registry.Resources[i].Name < registry.Resources[j].Name })
	return registry.Resources, nil
}

func Encode(resources []Definition) ([]byte, error) {
	values := append([]Definition(nil), resources...)
	sort.Slice(values, func(i, j int) bool { return values[i].Name < values[j].Name })
	return json.MarshalIndent(Registry{SchemaVersion: 1, Resources: values}, "", "  ")
}

func Add(resources []Definition, definition Definition) ([]Definition, bool, error) {
	for _, current := range resources {
		if current.Name == definition.Name {
			if fieldsString(current.Fields) == fieldsString(definition.Fields) {
				return resources, false, nil
			}
			return nil, false, fmt.Errorf("resource %q already exists with a different field definition", definition.Name)
		}
	}
	if definition.Migration == 0 {
		for _, current := range resources {
			if current.Migration >= definition.Migration {
				definition.Migration = current.Migration + 1
			}
		}
		if definition.Migration == 0 {
			definition.Migration = 1
		}
	}
	result := append(append([]Definition(nil), resources...), definition)
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, true, nil
}

func supportedType(value string) bool {
	switch value {
	case "uuid", "string", "int64", "integer", "decimal", "bool", "datetime":
		return true
	}
	return false
}

func pluralize(name string) string {
	if strings.HasSuffix(name, "y") && len(name) > 1 {
		return name[:len(name)-1] + "ies"
	}
	if strings.HasSuffix(name, "s") || strings.HasSuffix(name, "x") || strings.HasSuffix(name, "ch") || strings.HasSuffix(name, "sh") {
		return name + "es"
	}
	return name + "s"
}

func fieldsString(fields []Field) string {
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		values = append(values, field.Name+":"+field.Type)
	}
	return strings.Join(values, ",")
}

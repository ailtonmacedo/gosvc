package generator

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ailtonmacedo/gosvc/internal/preset"
	"github.com/ailtonmacedo/gosvc/internal/project"
	"github.com/ailtonmacedo/gosvc/internal/resource"
)

func BenchmarkRenderMinimalAPI(b *testing.B) {
	config := benchmarkConfig("minimal-api")
	definition, err := preset.Resolve("minimal-api")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		artifacts, err := Render(config, definition, nil)
		if err != nil {
			b.Fatal(err)
		}
		if len(artifacts) == 0 {
			b.Fatal("Render() returned no artifacts")
		}
	}
}

func BenchmarkRenderEventDrivenAPI(b *testing.B) {
	config := benchmarkConfig("event-driven-api")
	definition, err := preset.Resolve("event-driven-api")
	if err != nil {
		b.Fatal(err)
	}
	resources := []resource.Definition{resource.DefaultOrder()}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		artifacts, err := Render(config, definition, resources)
		if err != nil {
			b.Fatal(err)
		}
		if len(artifacts) == 0 {
			b.Fatal("Render() returned no artifacts")
		}
	}
}

func BenchmarkGenerateMinimalAPI(b *testing.B) {
	root := b.TempDir()
	config := benchmarkConfig("minimal-api")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		destination := filepath.Join(root, fmt.Sprintf("service-%d", i))
		if _, err := Generate(Request{Config: config, Destination: destination, FrameworkVersion: "benchmark"}); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkConfig(name string) project.Config {
	config := project.DefaultConfigForPreset(name)
	config.Project.Name = "benchmark-service"
	config.Project.Module = "github.com/gosvc/benchmark/service"
	if name == "production-api" || name == "event-driven-api" {
		config.Auth.AccessToken.Issuer = config.Project.Name
		config.Auth.AccessToken.Audience = config.Project.Name + "-api"
	}
	if name == "event-driven-api" {
		config.Deployment.Namespace = config.Project.Name
	}
	return config
}

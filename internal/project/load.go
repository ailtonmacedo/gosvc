package project

import (
	"fmt"
	"os"
)

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("open project configuration %q: %w", path, err)
	}
	root, err := parseSimpleYAML(data)
	if err != nil {
		return Config{}, fmt.Errorf("decode project configuration %q: %w", path, err)
	}
	config, err := decodeProjectConfig(root)
	if err != nil {
		return Config{}, fmt.Errorf("decode project configuration %q: %w", path, err)
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

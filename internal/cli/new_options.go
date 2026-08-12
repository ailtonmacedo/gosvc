package cli

import (
	"fmt"
	"strconv"
	"strings"
)

type newOptions struct {
	Name          string
	Module        string
	Preset        string
	PresetVersion string
	Router        string
	Persistence   string
	Destination   string
	DryRun        bool
	Force         bool
	Help          bool
}

func parseNewOptions(args []string) (newOptions, error) {
	options := newOptions{Preset: "minimal-api"}
	positionals := make([]string, 0, 1)

	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "-h" || argument == "--help" {
			options.Help = true
			continue
		}
		if argument == "--dry-run" {
			options.DryRun = true
			continue
		}
		if argument == "--force" {
			options.Force = true
			continue
		}
		for _, item := range []struct {
			prefix string
			set    func(string)
		}{
			{"--module=", func(value string) { options.Module = value }},
			{"--preset=", func(value string) { options.Preset = value }},
			{"--preset-version=", func(value string) { options.PresetVersion = value }},
			{"--router=", func(value string) { options.Router = value }},
			{"--persistence=", func(value string) { options.Persistence = value }},
			{"--output=", func(value string) { options.Destination = value }},
		} {
			if strings.HasPrefix(argument, item.prefix) {
				item.set(strings.TrimPrefix(argument, item.prefix))
				argument = ""
				break
			}
		}
		if argument == "" {
			continue
		}
		switch argument {
		case "--module", "--preset", "--preset-version", "--router", "--persistence", "--output":
			if index+1 >= len(args) {
				return newOptions{}, fmt.Errorf("%s requires a value", argument)
			}
			index++
			value := args[index]
			switch argument {
			case "--module":
				options.Module = value
			case "--preset":
				options.Preset = value
			case "--preset-version":
				options.PresetVersion = value
			case "--router":
				options.Router = value
			case "--persistence":
				options.Persistence = value
			case "--output":
				options.Destination = value
			}
		default:
			if strings.HasPrefix(argument, "-") {
				return newOptions{}, fmt.Errorf("unknown new flag %q", argument)
			}
			positionals = append(positionals, argument)
		}
	}

	if options.Help {
		return options, nil
	}
	if len(positionals) != 1 {
		return newOptions{}, fmt.Errorf("expected exactly one project name, got %s", strconv.Itoa(len(positionals)))
	}
	options.Name = positionals[0]
	if strings.TrimSpace(options.Module) == "" {
		return newOptions{}, fmt.Errorf("--module is required")
	}
	return options, nil
}

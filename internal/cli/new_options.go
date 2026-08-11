package cli

import (
	"fmt"
	"strconv"
	"strings"
)

type newOptions struct {
	Name        string
	Module      string
	Preset      string
	Destination string
	DryRun      bool
	Force       bool
	Help        bool
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
		if strings.HasPrefix(argument, "--module=") {
			options.Module = strings.TrimPrefix(argument, "--module=")
			continue
		}
		if strings.HasPrefix(argument, "--preset=") {
			options.Preset = strings.TrimPrefix(argument, "--preset=")
			continue
		}
		if strings.HasPrefix(argument, "--output=") {
			options.Destination = strings.TrimPrefix(argument, "--output=")
			continue
		}
		switch argument {
		case "--module", "--preset", "--output":
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
	if options.Destination == "" {
		options.Destination = options.Name
	}
	return options, nil
}

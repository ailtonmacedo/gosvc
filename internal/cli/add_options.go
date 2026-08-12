package cli

import (
	"flag"
	"fmt"
	"io"
)

type addResourceOptions struct {
	Name       string
	Fields     string
	ProjectDir string
	CRUD       bool
	DryRun     bool
	Force      bool
	Help       bool
}

func parseAddResourceOptions(args []string) (addResourceOptions, error) {
	var options addResourceOptions
	flags := flag.NewFlagSet("add resource", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.Fields, "fields", "", "comma-separated field definitions")
	flags.StringVar(&options.ProjectDir, "project", ".", "gosvc project directory")
	flags.BoolVar(&options.CRUD, "crud", false, "generate CRUD operations")
	flags.BoolVar(&options.DryRun, "dry-run", false, "show plan without writing")
	flags.BoolVar(&options.Force, "force", false, "overwrite modified generated files")
	flags.BoolVar(&options.Help, "help", false, "show help")
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--fields" || arg == "--project" {
			if i+1 >= len(args) {
				return options, fmt.Errorf("%s requires a value", arg)
			}
			if err := flags.Set(arg[2:], args[i+1]); err != nil {
				return options, err
			}
			i++
			continue
		}
		if arg == "--crud" || arg == "--dry-run" || arg == "--force" || arg == "--help" || arg == "-h" {
			name := arg[2:]
			if arg == "-h" {
				name = "help"
			}
			_ = flags.Set(name, "true")
			continue
		}
		if len(arg) > 0 && arg[0] == '-' {
			return options, fmt.Errorf("unknown flag %q", arg)
		}
		positionals = append(positionals, arg)
	}
	if options.Help {
		return options, nil
	}
	if len(positionals) != 1 {
		return options, fmt.Errorf("usage: gosvc add resource <name> --fields name:type,... --crud")
	}
	options.Name = positionals[0]
	if options.Fields == "" {
		return options, fmt.Errorf("--fields is required")
	}
	if !options.CRUD {
		return options, fmt.Errorf("--crud is required in the current version")
	}
	return options, nil
}

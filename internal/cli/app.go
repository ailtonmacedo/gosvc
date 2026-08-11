package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/acceptance"
	"github.com/ailtonmacedo/gosvc/internal/architecture"
	"github.com/ailtonmacedo/gosvc/internal/backup"
	"github.com/ailtonmacedo/gosvc/internal/buildinfo"
	"github.com/ailtonmacedo/gosvc/internal/certification"
	"github.com/ailtonmacedo/gosvc/internal/clierror"
	"github.com/ailtonmacedo/gosvc/internal/completion"
	"github.com/ailtonmacedo/gosvc/internal/doctor"
	"github.com/ailtonmacedo/gosvc/internal/generator"
	"github.com/ailtonmacedo/gosvc/internal/githubpublish"
	"github.com/ailtonmacedo/gosvc/internal/modulepath"
	"github.com/ailtonmacedo/gosvc/internal/plugin"
	"github.com/ailtonmacedo/gosvc/internal/pluginapply"
	"github.com/ailtonmacedo/gosvc/internal/project"
	"github.com/ailtonmacedo/gosvc/internal/projectcheck"
	"github.com/ailtonmacedo/gosvc/internal/releasecheck"
	"github.com/ailtonmacedo/gosvc/internal/releasepack"
	"github.com/ailtonmacedo/gosvc/internal/releaseverify"
	"github.com/ailtonmacedo/gosvc/internal/resource"
	"github.com/ailtonmacedo/gosvc/internal/upgrade"
	"github.com/ailtonmacedo/gosvc/internal/upgradenotes"
	"github.com/ailtonmacedo/gosvc/internal/verify"
)

type App struct {
	stdout io.Writer
	stderr io.Writer
}

func New(stdout, stderr io.Writer) *App {
	return &App{stdout: stdout, stderr: stderr}
}

func (a *App) Run(args []string) int {
	debug, args, err := parseGlobalFlags(args)
	if err != nil {
		a.printError(err, debug)
		return clierror.ExitCode(err)
	}

	if len(args) == 0 {
		a.printHelp()
		return 0
	}

	var runErr error
	switch args[0] {
	case "help", "-h", "--help":
		a.printHelp()
	case "version", "--version":
		runErr = a.runVersion(args[1:])
	case "validate-config":
		runErr = a.runValidateConfig(args[1:])
	case "validate":
		runErr = a.runValidate(args[1:])
	case "doctor":
		runErr = a.runDoctor(args[1:])
	case "check":
		runErr = a.runCheck(args[1:])
	case "verify":
		runErr = a.runVerify(args[1:])
	case "new":
		runErr = a.runNew(args[1:])
	case "add":
		runErr = a.runAdd(args[1:])
	case "upgrade":
		runErr = a.runUpgrade(args[1:])
	case "plugins":
		runErr = a.runPlugins(args[1:])
	case "completion":
		runErr = a.runCompletion(args[1:])
	case "release":
		runErr = a.runRelease(args[1:])
	case "acceptance":
		runErr = a.runAcceptance(args[1:])
	case "certify":
		runErr = a.runCertify(args[1:])
	default:
		runErr = clierror.New(clierror.CodeGeneral,
			fmt.Sprintf("unknown command %q; run 'gosvc --help'", args[0]))
	}

	if runErr != nil {
		a.printError(runErr, debug)
	}
	return clierror.ExitCode(runErr)
}

func parseGlobalFlags(args []string) (bool, []string, error) {
	debug := false
	remaining := make([]string, 0, len(args))

	for _, arg := range args {
		switch arg {
		case "--debug":
			debug = true
		case "--no-color":
			// Reserved. Output is currently color-free.
		default:
			remaining = append(remaining, arg)
		}
	}
	return debug, remaining, nil
}

func (a *App) printHelp() {
	fmt.Fprintln(a.stdout, `gosvc — opinionated Go service generator

Usage:
  gosvc [global flags] <command> [arguments]

Commands:
  version          Show build version information
  validate-config  Load, default, and validate a project.yaml
  validate         Validate a generated project and manifest
  doctor           Check required development tools
  check architecture  Enforce Clean Architecture import boundaries
  verify           Run structural and quality verification
  new              Create or safely regenerate a project scaffold
  add resource     Add a CRUD resource to a database-backed project
  upgrade          Upgrade templates and manifest schema safely
  plugins          List, validate, checksum, or run external plugins
  completion       Generate shell completion scripts
  release          Prepare, check, build, or verify release assets
  acceptance       Generate and validate the built-in preset matrix
  certify          Run static or real integration certification
  help             Show this help

Global flags:
  --debug          Show underlying error causes
  --no-color       Disable colored output
  --help           Show help
  --version        Show version

Examples:
  gosvc version
  gosvc validate-config ./project.yaml
  gosvc validate --project .
  gosvc doctor --project .
  gosvc check architecture --project .
  gosvc verify --project . --static
  gosvc new order-service --module github.com/acme/order-service --preset minimal-api
  gosvc add resource product --fields "id:uuid,name:string,price:decimal" --crud
  gosvc upgrade --project . --dry-run
  gosvc upgrade backups --project .
  gosvc upgrade rollback --project . --backup latest --dry-run
  gosvc upgrade notes --from 1.0.0 --to 1.1.0
  gosvc plugins list --project .
  gosvc plugins checksum .gosvc/plugins/audit/bin/audit
  gosvc plugins run audit --project . --dry-run
  gosvc completion bash
  gosvc release prepare --repository ailtonmacedo/gosvc --dry-run
  gosvc release github-plan --repository ailtonmacedo/gosvc --version 1.1.0
  gosvc release check --version 1.0.0
  gosvc release snapshot --version 1.0.0 --output dist --parallel 3
  gosvc release verify --dist dist
  gosvc acceptance --json
  gosvc certify --mode real --json`)
}

func (a *App) runVersion(args []string) error {
	if len(args) > 0 {
		return clierror.New(clierror.CodeGeneral, "version does not accept arguments")
	}
	fmt.Fprintf(a.stdout, "gosvc version %s\ncommit: %s\nbuilt: %s\n",
		buildinfo.Version, buildinfo.Commit, buildinfo.BuildTime)
	return nil
}

func (a *App) runValidateConfig(args []string) error {
	flags := flag.NewFlagSet("validate-config", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	showResolved := flags.Bool("print", false, "print resolved configuration summary")

	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig,
			"invalid validate-config arguments", err)
	}
	if flags.NArg() != 1 {
		return clierror.New(clierror.CodeInvalidConfig,
			"usage: gosvc validate-config [--print] <project.yaml>")
	}

	config, err := project.Load(flags.Arg(0))
	if err != nil {
		code := clierror.CodeInvalidConfig
		return clierror.Wrap(code, "project configuration is invalid", err)
	}

	fmt.Fprintf(a.stdout, "Configuration is valid: %s\n", flags.Arg(0))
	if *showResolved {
		fmt.Fprintln(a.stdout, config.Summary())
	}
	return nil
}

func (a *App) runNew(args []string) error {
	options, err := parseNewOptions(args)
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid new arguments", err)
	}
	if options.Help {
		fmt.Fprintln(a.stdout, `Usage:
  gosvc new <project-name> --module <module-path> [options]

Options:
  --preset <name>   Project preset: minimal-api, postgres-api, production-api, or event-driven-api
  --output <path>   Destination directory; defaults to the project name
  --dry-run         Show the plan without writing files
  --force           Overwrite conflicting managed files explicitly

Flags may appear before or after the project name.`)
		return nil
	}

	config := project.DefaultConfigForPreset(options.Preset)
	config.Project.Name = options.Name
	config.Project.Module = options.Module
	if options.Preset == "production-api" || options.Preset == "event-driven-api" {
		config.Auth.AccessToken.Issuer = options.Name
		config.Auth.AccessToken.Audience = options.Name + "-api"
	}
	if options.Preset == "event-driven-api" {
		config.Deployment.Namespace = options.Name
	}
	if err := config.Validate(); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "project configuration is invalid", err)
	}

	result, err := generator.Generate(generator.Request{
		Config:           config,
		Destination:      options.Destination,
		DryRun:           options.DryRun,
		Force:            options.Force,
		FrameworkVersion: buildinfo.Version,
	})
	if err != nil {
		code := clierror.CodeGenerationFailure
		if generator.IsConflict(err) {
			code = clierror.CodeFileConflict
		}
		return clierror.Wrap(code, "generate project", err)
	}

	for _, change := range result.Changes {
		if change.Reason == "" {
			fmt.Fprintf(a.stdout, "%-7s %s\n", change.Action, change.Artifact.Path)
			continue
		}
		fmt.Fprintf(a.stdout, "%-7s %s (%s)\n", change.Action, change.Artifact.Path, change.Reason)
	}
	fmt.Fprintf(a.stdout, "%-7s %s\n", result.ManifestAction, ".gosvc/manifest.json")
	if options.DryRun {
		fmt.Fprintf(a.stdout, "Dry run complete for %s; no files were written.\n", result.Destination)
		return nil
	}
	if !result.Applied {
		fmt.Fprintln(a.stdout, "No changes required.")
		return nil
	}
	fmt.Fprintf(a.stdout, "Project generated at %s using preset %s.\n", result.Destination, result.Preset.Name)
	return nil
}

func (a *App) printError(err error, debug bool) {
	fmt.Fprintf(a.stderr, "error: %s\n", err)
	if !debug {
		return
	}

	for cause := errors.Unwrap(err); cause != nil; cause = errors.Unwrap(cause) {
		fmt.Fprintf(a.stderr, "caused by: %s\n", cause)
	}
}

func (a *App) runAdd(args []string) error {
	if len(args) == 0 || args[0] != "resource" {
		return clierror.New(clierror.CodeGeneral, "usage: gosvc add resource <name> --fields name:type,... --crud")
	}
	options, err := parseAddResourceOptions(args[1:])
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid add resource arguments", err)
	}
	if options.Help {
		fmt.Fprintln(a.stdout, `Usage:
  gosvc add resource <name> --fields <name:type,...> --crud [options]

Options:
  --project <path>  gosvc project directory; defaults to current directory
  --dry-run         Show changes without writing files
  --force           Overwrite conflicting generated files explicitly

Supported field types: uuid, string, int64, integer, decimal, bool, datetime`)
		return nil
	}
	definition, err := resource.Parse(options.Name, options.Fields)
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid resource definition", err)
	}
	result, added, err := generator.AddResource(generator.AddResourceRequest{
		ProjectDir: options.ProjectDir, Definition: definition, DryRun: options.DryRun,
		Force: options.Force, FrameworkVersion: buildinfo.Version,
	})
	if err != nil {
		code := clierror.CodeGenerationFailure
		if generator.IsConflict(err) {
			code = clierror.CodeFileConflict
		}
		return clierror.Wrap(code, "add resource", err)
	}
	for _, change := range result.Changes {
		if change.Reason == "" {
			fmt.Fprintf(a.stdout, "%-7s %s\n", change.Action, change.Artifact.Path)
		} else {
			fmt.Fprintf(a.stdout, "%-7s %s (%s)\n", change.Action, change.Artifact.Path, change.Reason)
		}
	}
	fmt.Fprintf(a.stdout, "%-7s %s\n", result.ManifestAction, ".gosvc/manifest.json")
	if options.DryRun {
		fmt.Fprintln(a.stdout, "Dry run complete; no files were written.")
		return nil
	}
	if !added && !result.Applied {
		fmt.Fprintf(a.stdout, "Resource %s already exists; no changes required.\n", definition.Name)
		return nil
	}
	fmt.Fprintf(a.stdout, "Resource %s added to %s.\n", definition.Name, result.Destination)
	return nil
}

func (a *App) runValidate(args []string) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectDir := flags.String("project", ".", "gosvc project directory")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid validate arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc validate [--project <path>]")
	}
	report, err := projectcheck.Check(*projectDir)
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "validate project", err)
	}
	for _, issue := range report.Issues {
		fmt.Fprintln(a.stdout, issue.String())
	}
	if err := report.Error(); err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "validate project", err)
	}
	fmt.Fprintf(a.stdout, "Project is valid: files=%d resources=%d architecture_files=%d\n", report.FilesChecked, report.ResourcesChecked, report.ArchitectureChecked)
	return nil
}

func (a *App) runDoctor(args []string) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectDir := flags.String("project", ".", "gosvc project directory")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid doctor arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc doctor [--project <path>]")
	}
	report, err := doctor.Check(*projectDir)
	if err != nil {
		return clierror.Wrap(clierror.CodeMissingDependency, "inspect development environment", err)
	}
	for _, result := range report.Results {
		details := result.Version
		if result.Error != "" {
			if details != "" {
				details += " (" + result.Error + ")"
			} else {
				details = result.Error
			}
		}
		fmt.Fprintf(a.stdout, "%-8s %-18s %s\n", result.Status, result.Tool.Name, details)
	}
	if err := report.Err(); err != nil {
		return clierror.Wrap(clierror.CodeMissingDependency, "development environment is incomplete", err)
	}
	fmt.Fprintln(a.stdout, "Development environment is ready.")
	return nil
}

func (a *App) runCheck(args []string) error {
	if len(args) == 0 || args[0] != "architecture" {
		return clierror.New(clierror.CodeGeneral, "usage: gosvc check architecture [--project <path>]")
	}
	flags := flag.NewFlagSet("check architecture", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectDir := flags.String("project", ".", "gosvc project directory")
	if err := flags.Parse(args[1:]); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid architecture check arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc check architecture [--project <path>]")
	}
	config, err := project.Load(filepath.Join(*projectDir, "project.yaml"))
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "load project configuration", err)
	}
	report, err := architecture.Check(*projectDir, config.Project.Module)
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "check architecture", err)
	}
	for _, violation := range report.Violations {
		fmt.Fprintln(a.stdout, violation.Error())
	}
	if err := report.Err(); err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "architecture violations found", err)
	}
	fmt.Fprintf(a.stdout, "Architecture is valid: %d files checked.\n", report.FilesChecked)
	return nil
}

func (a *App) runVerify(args []string) error {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectDir := flags.String("project", ".", "gosvc project directory")
	staticOnly := flags.Bool("static", false, "only validate generated structure and architecture")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid verify arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc verify [--project <path>] [--static]")
	}
	if err := verify.Run(verify.Options{ProjectDir: *projectDir, StaticOnly: *staticOnly, Output: a.stdout}); err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "verify project", err)
	}
	fmt.Fprintln(a.stdout, "Verification completed successfully.")
	return nil
}

func (a *App) runUpgrade(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "backups":
			return a.runUpgradeBackups(args[1:])
		case "rollback":
			return a.runUpgradeRollback(args[1:])
		case "notes":
			return a.runUpgradeNotes(args[1:])
		}
	}
	flags := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectDir := flags.String("project", ".", "gosvc project directory")
	target := flags.String("to", "current", "target gosvc version")
	dryRun := flags.Bool("dry-run", false, "show the upgrade plan without writing files")
	force := flags.Bool("force", false, "overwrite conflicting managed files")
	noBackup := flags.Bool("no-backup", false, "apply without creating a rollback backup")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid upgrade arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc upgrade [--project <path>] [--to <version>] [--dry-run] [--force] [--no-backup]")
	}
	result, err := upgrade.Run(upgrade.Options{
		ProjectDir: *projectDir, TargetVersion: *target,
		RuntimeVersion: buildinfo.Version, DryRun: *dryRun, Force: *force, NoBackup: *noBackup,
	})
	if err != nil {
		code := clierror.CodeGenerationFailure
		if generator.IsConflict(err) {
			code = clierror.CodeFileConflict
		}
		return clierror.Wrap(code, "upgrade project", err)
	}
	fmt.Fprintf(a.stdout, "Framework: %s -> %s\n", displayVersion(result.FromVersion), displayVersion(result.ToVersion))
	fmt.Fprintf(a.stdout, "Manifest schema: %d -> %d\n", result.FromSchemaVersion, result.ToSchemaVersion)
	for _, change := range result.Changes {
		if change.Reason == "" {
			fmt.Fprintf(a.stdout, "%-7s %s\n", change.Action, change.Artifact.Path)
		} else {
			fmt.Fprintf(a.stdout, "%-7s %s (%s)\n", change.Action, change.Artifact.Path, change.Reason)
		}
	}
	fmt.Fprintf(a.stdout, "%-7s %s\n", result.ManifestAction, ".gosvc/manifest.json")
	if *dryRun {
		fmt.Fprintln(a.stdout, "Upgrade dry run complete; no files were written.")
		return nil
	}
	if !result.UpgradeRequired {
		fmt.Fprintln(a.stdout, "Project is already up to date.")
		return nil
	}
	if result.BackupPath != "" {
		fmt.Fprintf(a.stdout, "Backup: %s\n", result.BackupPath)
	}
	fmt.Fprintf(a.stdout, "Project upgraded to %s.\n", displayVersion(result.ToVersion))
	return nil
}

func (a *App) runUpgradeBackups(args []string) error {
	flags := flag.NewFlagSet("upgrade backups", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectDir := flags.String("project", ".", "gosvc project directory")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid upgrade backups arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc upgrade backups [--project <path>]")
	}
	entries, err := backup.List(*projectDir)
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "list upgrade backups", err)
	}
	if len(entries) == 0 {
		fmt.Fprintln(a.stdout, "No upgrade backups found.")
		return nil
	}
	for _, entry := range entries {
		fmt.Fprintf(a.stdout, "%s\t%s\t%s\t%d bytes\n", entry.Metadata.CreatedAt, entry.Metadata.FrameworkVersion, entry.Path, entry.Size)
	}
	return nil
}

func (a *App) runUpgradeRollback(args []string) error {
	flags := flag.NewFlagSet("upgrade rollback", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectDir := flags.String("project", ".", "gosvc project directory")
	name := flags.String("backup", "latest", "backup filename or latest")
	dryRun := flags.Bool("dry-run", false, "show the selected backup without restoring it")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid upgrade rollback arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc upgrade rollback [--project <path>] [--backup <name|latest>] [--dry-run]")
	}
	entry, err := backup.Resolve(*projectDir, *name)
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "resolve upgrade backup", err)
	}
	fmt.Fprintf(a.stdout, "Restore %s created at %s (framework %s).\n", entry.Path, entry.Metadata.CreatedAt, entry.Metadata.FrameworkVersion)
	if *dryRun {
		fmt.Fprintln(a.stdout, "Rollback dry run complete; no files were written.")
		return nil
	}
	if err := backup.Restore(*projectDir, entry, time.Now()); err != nil {
		return clierror.Wrap(clierror.CodeGenerationFailure, "rollback project", err)
	}
	fmt.Fprintln(a.stdout, "Rollback completed successfully.")
	return nil
}

func (a *App) runUpgradeNotes(args []string) error {
	flags := flag.NewFlagSet("upgrade notes", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	from := flags.String("from", "1.0.0", "source gosvc version")
	to := flags.String("to", displayVersion(buildinfo.Version), "target gosvc version")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid upgrade notes arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc upgrade notes [--from <version>] [--to <version>]")
	}
	content, err := upgradenotes.Markdown(*from, *to)
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "render upgrade notes", err)
	}
	fmt.Fprint(a.stdout, content)
	return nil
}

func displayVersion(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func (a *App) runPlugins(args []string) error {
	if len(args) == 0 || (args[0] != "list" && args[0] != "validate" && args[0] != "checksum" && args[0] != "run") {
		return clierror.New(clierror.CodeGeneral, "usage: gosvc plugins <list|validate|checksum|run> [arguments]")
	}
	action := args[0]
	if action == "checksum" {
		if len(args) != 2 {
			return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc plugins checksum <entrypoint>")
		}
		content, err := os.ReadFile(args[1])
		if err != nil {
			return clierror.Wrap(clierror.CodeInvalidProject, "read plugin entrypoint", err)
		}
		fmt.Fprintln(a.stdout, plugin.Checksum(content))
		return nil
	}
	if action == "run" {
		return a.runExternalPlugin(args[1:])
	}
	flags := flag.NewFlagSet("plugins "+action, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectDir := flags.String("project", ".", "gosvc project directory")
	if err := flags.Parse(args[1:]); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid plugins arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc plugins <list|validate> [--project <path>]")
	}
	plugins, err := plugin.Discover(*projectDir, buildinfo.Version)
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "inspect plugins", err)
	}
	ready := 0
	for _, item := range plugins {
		source := item.Source
		status := "builtin"
		if item.BuiltIn {
			source = "builtin"
			ready++
		} else if item.SchemaVersion < plugin.CurrentSchemaVersion {
			status = "legacy"
		} else if action == "validate" {
			if _, entryErr := item.EntrypointPath(*projectDir); entryErr != nil {
				return clierror.Wrap(clierror.CodeInvalidProject, "validate plugin "+item.Name, entryErr)
			}
			status = "ready"
			ready++
		} else {
			status = "declared"
		}
		fmt.Fprintf(a.stdout, "%-16s %-12s %-9s %-10s %s\n", item.Name, item.Version, status, source, item.Description)
	}
	if action == "validate" {
		fmt.Fprintf(a.stdout, "Plugin manifests are valid: %d discovered, %d execution-ready.\n", len(plugins), ready)
	}
	return nil
}

func (a *App) runExternalPlugin(args []string) error {
	options, err := parsePluginRunOptions(args)
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid plugins run arguments", err)
	}
	result, err := pluginapply.Run(context.Background(), pluginapply.Options{
		ProjectDir: options.projectDir, PluginName: options.name,
		FrameworkVersion: buildinfo.Version, DryRun: options.dryRun,
		Force: options.force, Timeout: options.timeout,
	})
	if err != nil {
		code := clierror.CodeGenerationFailure
		if generator.IsConflict(err) {
			code = clierror.CodeFileConflict
		}
		return clierror.Wrap(code, "run plugin "+options.name, err)
	}
	for _, diagnostic := range result.Diagnostics {
		location := ""
		if diagnostic.Path != "" {
			location = diagnostic.Path + ": "
		}
		fmt.Fprintf(a.stdout, "%-7s %s%s\n", strings.ToUpper(diagnostic.Severity), location, diagnostic.Message)
	}
	for _, change := range result.Changes {
		if change.Reason == "" {
			fmt.Fprintf(a.stdout, "%-7s %s\n", change.Action, change.Artifact.Path)
		} else {
			fmt.Fprintf(a.stdout, "%-7s %s (%s)\n", change.Action, change.Artifact.Path, change.Reason)
		}
	}
	fmt.Fprintf(a.stdout, "%-7s %s\n", result.ManifestAction, ".gosvc/manifest.json")
	if options.dryRun {
		fmt.Fprintln(a.stdout, "Plugin dry run complete; no files were written.")
	} else if !result.Applied {
		fmt.Fprintln(a.stdout, "No changes required.")
	} else {
		fmt.Fprintf(a.stdout, "Plugin %s %s applied.\n", result.Plugin.Name, result.Plugin.Version)
	}
	return nil
}

type pluginRunOptions struct {
	name       string
	projectDir string
	dryRun     bool
	force      bool
	timeout    time.Duration
}

func parsePluginRunOptions(args []string) (pluginRunOptions, error) {
	options := pluginRunOptions{projectDir: ".", timeout: plugin.DefaultTimeout}
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch argument {
		case "--dry-run":
			options.dryRun = true
		case "--force":
			options.force = true
		case "--project", "--timeout":
			if index+1 >= len(args) {
				return pluginRunOptions{}, fmt.Errorf("%s requires a value", argument)
			}
			index++
			value := args[index]
			if argument == "--project" {
				options.projectDir = value
			} else {
				duration, err := time.ParseDuration(value)
				if err != nil || duration <= 0 || duration > time.Minute {
					return pluginRunOptions{}, fmt.Errorf("--timeout must be a duration greater than zero and no more than 1m")
				}
				options.timeout = duration
			}
		default:
			if strings.HasPrefix(argument, "-") {
				return pluginRunOptions{}, fmt.Errorf("unknown flag %q", argument)
			}
			if options.name != "" {
				return pluginRunOptions{}, fmt.Errorf("unexpected argument %q", argument)
			}
			options.name = argument
		}
	}
	if options.name == "" {
		return pluginRunOptions{}, fmt.Errorf("usage: gosvc plugins run <name> [--project <path>] [--dry-run] [--force] [--timeout 10s]")
	}
	return options, nil
}

func (a *App) runCompletion(args []string) error {
	if len(args) != 1 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc completion <bash|zsh|fish|powershell>")
	}
	content, err := completion.Generate(args[0])
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "generate completion", err)
	}
	fmt.Fprint(a.stdout, content)
	return nil
}

func (a *App) runAcceptance(args []string) error {
	flags := flag.NewFlagSet("acceptance", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workDir := flags.String("workdir", "", "empty directory used for generated acceptance projects")
	keep := flags.Bool("keep", false, "keep the temporary acceptance workspace")
	jsonOutput := flags.Bool("json", false, "emit the acceptance report as JSON")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid acceptance arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc acceptance [--workdir <empty-path>] [--keep] [--json]")
	}
	report, err := acceptance.Run(acceptance.Options{
		WorkDir: *workDir, Keep: *keep, JSON: *jsonOutput, Output: a.stdout,
		FrameworkVersion: buildinfo.Version,
	})
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "acceptance matrix", err)
	}
	if !*jsonOutput {
		fmt.Fprintf(a.stdout, "All built-in presets passed acceptance: %d/%d.\n", report.Passed, len(report.Presets))
	}
	return nil
}

func (a *App) runCertify(args []string) error {
	flags := flag.NewFlagSet("certify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", "static", "certification mode: static or real")
	workdir := flags.String("workdir", "", "preserve generated certification projects in this empty directory")
	keep := flags.Bool("keep", false, "keep the temporary certification workspace")
	jsonOutput := flags.Bool("json", false, "emit the certification report as JSON")
	requireReal := flags.Bool("require-real", false, "return a non-zero exit code when real certification is blocked")
	timeout := flags.Duration("timeout", 2*time.Minute, "timeout for each external certification command")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid certify arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc certify [--mode static|real] [--workdir <dir>] [--keep] [--json] [--require-real] [--timeout 2m]")
	}
	_, err := certification.Run(certification.Options{
		Mode: certification.Mode(*mode), WorkDir: *workdir, Keep: *keep,
		JSON: *jsonOutput, Output: a.stdout, FrameworkVersion: buildinfo.Version,
		RequireReal: *requireReal, CommandTimeout: *timeout,
	})
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "certification failed", err)
	}
	return nil
}

func (a *App) runRelease(args []string) error {
	if len(args) == 0 {
		return clierror.New(clierror.CodeGeneral, "usage: gosvc release <prepare|github-plan|check|snapshot|verify> [arguments]")
	}
	switch args[0] {
	case "prepare":
		return a.runReleasePrepare(args[1:])
	case "github-plan":
		return a.runReleaseGitHubPlan(args[1:])
	case "verify":
		return a.runReleaseVerify(args[1:])
	case "check", "snapshot":
		return a.runReleaseBuildAction(args[0], args[1:])
	default:
		return clierror.New(clierror.CodeGeneral, "usage: gosvc release <prepare|github-plan|check|snapshot|verify> [arguments]")
	}
}

func (a *App) runReleasePrepare(args []string) error {
	flags := flag.NewFlagSet("release prepare", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectDir := flags.String("project", ".", "gosvc repository directory")
	repositoryValue := flags.String("repository", "", "GitHub repository in owner/name notation")
	dryRun := flags.Bool("dry-run", false, "show changes without writing files")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid release prepare arguments", err)
	}
	if flags.NArg() != 0 || *repositoryValue == "" {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc release prepare --repository <owner/name> [--project <path>] [--dry-run]")
	}
	plan, err := modulepath.Prepare(*projectDir, *repositoryValue)
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "prepare release repository", err)
	}
	fmt.Fprintf(a.stdout, "Module: %s -> %s\n", plan.OldModule, plan.NewModule)
	for _, change := range plan.Changes {
		fmt.Fprintf(a.stdout, "%-7s %s (%d replacements)\n", "UPDATE", change.Path, change.Replacements)
	}
	if len(plan.Changes) == 0 {
		fmt.Fprintln(a.stdout, "Repository identity is already prepared; no changes required.")
		return nil
	}
	if *dryRun {
		fmt.Fprintf(a.stdout, "Dry run complete: %d files would be updated.\n", len(plan.Changes))
		return nil
	}
	if err := modulepath.Apply(plan); err != nil {
		return clierror.Wrap(clierror.CodeGenerationFailure, "apply repository identity", err)
	}
	fmt.Fprintf(a.stdout, "Repository prepared for %s; %d files updated.\n", plan.Repository.Slug(), len(plan.Changes))
	return nil
}

func (a *App) runReleaseGitHubPlan(args []string) error {
	flags := flag.NewFlagSet("release github-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectDir := flags.String("project", ".", "gosvc repository directory")
	repositoryValue := flags.String("repository", "", "GitHub repository in owner/name notation")
	versionValue := flags.String("version", "1.1.0", "publication semantic version")
	jsonOutput := flags.Bool("json", false, "emit the publication plan as JSON")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid release github-plan arguments", err)
	}
	if flags.NArg() != 0 || *repositoryValue == "" {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc release github-plan --repository <owner/name> [--version <version>] [--project <path>] [--json]")
	}
	plan, err := githubpublish.Build(githubpublish.Options{Root: *projectDir, Repository: *repositoryValue, Version: *versionValue})
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "build GitHub publication plan", err)
	}
	if *jsonOutput {
		encoded, err := plan.EncodeJSON()
		if err != nil {
			return clierror.Wrap(clierror.CodeGeneral, "encode GitHub publication plan", err)
		}
		fmt.Fprintln(a.stdout, string(encoded))
	} else {
		fmt.Fprintf(a.stdout, "GitHub publication plan: repository=%s module=%s version=%s\n", plan.Repository, plan.Module, plan.Version)
		for _, check := range plan.Checks {
			fmt.Fprintf(a.stdout, "%-7s %-24s %s\n", strings.ToUpper(string(check.Status)), check.Name, check.Detail)
		}
		fmt.Fprintln(a.stdout, "\nPublication sequence:")
		for _, step := range plan.Steps {
			fmt.Fprintf(a.stdout, "%2d. %-42s %s\n", step.Order, step.Description, step.Command)
		}
		fmt.Fprintf(a.stdout, "\nReadiness: passed=%d warnings=%d failed=%d\n", plan.Passed, plan.Warnings, plan.Failed)
	}
	if !plan.Ready() {
		return clierror.New(clierror.CodeInvalidProject, "GitHub publication plan has blocking failures: "+strings.Join(githubpublish.SortedFailures(plan), "; "))
	}
	return nil
}

func (a *App) runReleaseVerify(args []string) error {
	flags := flag.NewFlagSet("release verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dist := flags.String("dist", "dist", "release asset directory")
	skipExec := flags.Bool("skip-exec", false, "do not execute the host-platform binary")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid release verify arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc release verify [--dist <path>] [--skip-exec]")
	}
	report, err := releaseverify.Verify(releaseverify.Options{Dist: *dist, Execute: !*skipExec})
	if err != nil {
		return clierror.Wrap(clierror.CodeInvalidProject, "verify release assets", err)
	}
	status := "archive and metadata checks passed"
	if report.Executed {
		status += "; host binary executed successfully"
	}
	fmt.Fprintf(a.stdout, "Release assets are valid: version=%s repository=%s assets=%d; %s.\n",
		report.Manifest.Version, report.Manifest.Repository, report.Verified, status)
	return nil
}

func (a *App) runReleaseBuildAction(action string, args []string) error {
	flags := flag.NewFlagSet("release "+action, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectDir := flags.String("project", ".", "gosvc repository directory")
	versionValue := flags.String("version", buildinfo.Version, "release semantic version")
	repositoryValue := flags.String("repository", "", "GitHub repository in owner/name notation")
	allowPlaceholder := flags.Bool("allow-placeholder", false, "allow github.com/example module for local snapshots")
	output := flags.String("output", "dist", "release asset directory")
	parallel := flags.Int("parallel", 0, "parallel cross-platform builds; defaults to available CPUs")
	if err := flags.Parse(args); err != nil {
		return clierror.Wrap(clierror.CodeInvalidConfig, "invalid release arguments", err)
	}
	if flags.NArg() != 0 {
		return clierror.New(clierror.CodeInvalidConfig, "usage: gosvc release "+action+" [--project <path>] --version <version> [--repository <owner/name>] [--output <path>] [--parallel <n>] [--allow-placeholder]")
	}
	if *parallel < 0 || *parallel > 32 {
		return clierror.New(clierror.CodeInvalidConfig, "--parallel must be between 0 and 32")
	}
	if action == "check" {
		report, err := releasecheck.Check(releasecheck.Options{Root: *projectDir, Version: *versionValue, Repository: *repositoryValue, AllowPlaceholder: *allowPlaceholder})
		if err != nil {
			return clierror.Wrap(clierror.CodeInvalidProject, "release preflight", err)
		}
		for _, issue := range report.Issues {
			fmt.Fprintf(a.stdout, "error    %s\n", issue)
		}
		if err := report.Err(); err != nil {
			return clierror.Wrap(clierror.CodeInvalidProject, "release preflight failed", err)
		}
		fmt.Fprintf(a.stdout, "Release preflight passed: module=%s repository=%s version=%s\n", report.Module, report.Repository, report.Version)
		return nil
	}
	result, err := releasepack.Build(releasepack.Options{
		Root: *projectDir, Output: *output, Version: *versionValue,
		Repository: *repositoryValue, AllowPlaceholder: *allowPlaceholder, Parallel: *parallel,
	})
	if err != nil {
		return clierror.Wrap(clierror.CodeGenerationFailure, "build release snapshot", err)
	}
	for _, asset := range result.Assets {
		fmt.Fprintf(a.stdout, "%-12d %s\n", asset.Size, asset.Name)
	}
	fmt.Fprintf(a.stdout, "Release snapshot %s created at %s (%d assets).\n", result.Version, result.OutputDir, len(result.Assets))
	return nil
}

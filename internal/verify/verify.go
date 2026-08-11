package verify

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ailtonmacedo/gosvc/internal/projectcheck"
)

type Options struct {
	ProjectDir string
	StaticOnly bool
	Output     io.Writer
}

func Run(options Options) error {
	if options.ProjectDir == "" {
		options.ProjectDir = "."
	}
	if options.Output == nil {
		options.Output = io.Discard
	}
	absolute, err := filepath.Abs(options.ProjectDir)
	if err != nil {
		return fmt.Errorf("resolve project directory: %w", err)
	}
	report, err := projectcheck.Check(absolute)
	if err != nil {
		return err
	}
	for _, issue := range report.Issues {
		fmt.Fprintln(options.Output, issue.String())
	}
	if err := report.Error(); err != nil {
		return err
	}
	fmt.Fprintln(options.Output, "PASS    project structure")
	if options.StaticOnly {
		return nil
	}
	commands := [][]string{
		{"go", "test", "./..."},
		{"go", "test", "-race", "./..."},
		{"go", "vet", "./..."},
		{"go", "build", "./..."},
		{"golangci-lint", "run"},
		{"govulncheck", "./..."},
	}
	for _, command := range commands {
		if err := runCommand(absolute, options.Output, command[0], command[1:]...); err != nil {
			return err
		}
	}
	return nil
}

func runCommand(dir string, output io.Writer, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Stdout = output
	command.Stderr = output
	command.Env = os.Environ()
	fmt.Fprintf(output, "RUN     %s %v\n", name, args)
	if err := command.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	fmt.Fprintf(output, "PASS    %s\n", name)
	return nil
}

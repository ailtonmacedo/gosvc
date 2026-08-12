package clierror

import (
	"errors"
	"testing"
)

func TestExitCode(t *testing.T) {
	t.Parallel()

	cause := errors.New("bad yaml")
	err := Wrap(CodeInvalidConfig, "invalid config", cause)

	if got := ExitCode(err); got != int(CodeInvalidConfig) {
		t.Fatalf("ExitCode() = %d, want %d", got, CodeInvalidConfig)
	}
	if !errors.Is(err, cause) {
		t.Fatal("wrapped error does not preserve cause")
	}
}

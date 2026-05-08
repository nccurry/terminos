// SPDX-License-Identifier: AGPL-3.0-or-later

package apperror

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExitCodeMapsApplicationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: ExitSuccess},
		{name: "usage", err: New(CodeUsage, "bad args"), want: ExitUsage},
		{name: "validation", err: New(CodeValidation, "invalid"), want: ExitValidation},
		{name: "captured command", err: New(CodeCapturedCommand, "failed"), want: ExitCapturedCommand},
		{name: "timeout", err: New(CodeTimeout, "slow"), want: ExitTimeout},
		{name: "filesystem", err: New(CodeFilesystem, "missing"), want: ExitFilesystem},
		{name: "redaction", err: New(CodeRedactionPolicy, "secret"), want: ExitRedactionPolicy},
		{name: "context canceled", err: context.Canceled, want: ExitInterrupted},
		{name: "deadline", err: context.DeadlineExceeded, want: ExitTimeout},
		{name: "plain", err: errors.New("plain"), want: ExitInternal},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := ExitCode(test.err); got != test.want {
				t.Fatalf("ExitCode() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestWrapIncludesMessageAndCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("disk full")
	err := Wrap(CodeFilesystem, "write file", cause)

	if !errors.Is(err, cause) {
		t.Fatal("expected wrapped cause")
	}
	if !strings.Contains(err.Error(), "write file: disk full") {
		t.Fatalf("unexpected error string %q", err.Error())
	}
	if ExitCode(err) != ExitFilesystem {
		t.Fatalf("unexpected exit code %d", ExitCode(err))
	}
}

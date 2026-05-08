// SPDX-License-Identifier: AGPL-3.0-or-later

package redact

import (
	"testing"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/model"
)

func TestRuleSetAppliesRulesInOrder(t *testing.T) {
	t.Parallel()

	rules, err := New([]model.Redaction{
		{Pattern: `token=[a-z0-9]+`, Replacement: "token=REDACTED"},
		{Pattern: `/home/[^ ]+`, Replacement: "~"},
	})
	if err != nil {
		t.Fatalf("compile redactions: %v", err)
	}

	got := rules.Apply("token=abc123 /home/user/project")
	want := "token=REDACTED ~"
	if got != want {
		t.Fatalf("Apply() = %q, want %q", got, want)
	}
}

func TestNewRejectsInvalidPattern(t *testing.T) {
	t.Parallel()

	_, err := New([]model.Redaction{{Pattern: "["}})
	if apperror.ExitCode(err) != apperror.ExitUsage {
		t.Fatalf("expected usage exit, got %d from %v", apperror.ExitCode(err), err)
	}
}

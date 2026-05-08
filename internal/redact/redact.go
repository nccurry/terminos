// SPDX-License-Identifier: AGPL-3.0-or-later

// Package redact applies manifest redaction rules.
package redact

import (
	"regexp"

	"github.com/nccurry/terminos/internal/apperror"
	"github.com/nccurry/terminos/internal/model"
)

// RuleSet applies ordered regex replacements.
type RuleSet struct {
	rules []rule
}

type rule struct {
	pattern     *regexp.Regexp
	replacement string
}

// New compiles redaction rules from a manifest.
func New(redactions []model.Redaction) (RuleSet, error) {
	rules := make([]rule, 0, len(redactions))
	for _, item := range redactions {
		pattern, err := regexp.Compile(item.Pattern)
		if err != nil {
			return RuleSet{}, apperror.Wrapf(apperror.CodeUsage, err, "compile redaction pattern %q", item.Pattern)
		}
		rules = append(rules, rule{pattern: pattern, replacement: item.Replacement})
	}
	return RuleSet{rules: rules}, nil
}

// Apply returns value after applying each redaction rule in order.
func (r RuleSet) Apply(value string) string {
	for _, rule := range r.rules {
		value = rule.pattern.ReplaceAllString(value, rule.replacement)
	}
	return value
}

package ruleset

import "errors"

var (
	// ErrInvalidRuleset indicates the ruleset structure is invalid
	ErrInvalidRuleset = errors.New("invalid ruleset")

	// ErrInvalidRulesetType indicates the ruleset type doesn't match expected
	ErrInvalidRulesetType = errors.New("invalid ruleset type")

	// ErrRulesetNotFound indicates the ruleset was not found
	ErrRulesetNotFound = errors.New("ruleset not found")

	// ErrHashMismatch indicates the ruleset hash doesn't match
	ErrHashMismatch = errors.New("ruleset hash mismatch")

	// ErrRulesetDeprecated indicates the ruleset has been deprecated
	ErrRulesetDeprecated = errors.New("ruleset is deprecated")

	// ErrInvalidCondition indicates a rule condition is invalid
	ErrInvalidCondition = errors.New("invalid rule condition")

	// ErrUnsupportedConditionType indicates the condition type is not supported
	ErrUnsupportedConditionType = errors.New("unsupported condition type")
)

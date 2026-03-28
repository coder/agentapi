package model

import (
	"maps"
	"regexp"
	"slices"
)

// ConfigBuilder defines a configuration builder.
type ConfigBuilder interface {
	// SetOverride sets the configuration override.
	SetOverride(override *ConfigOverride)

	// GetConfig builds and returns the configuration object for the given path.
	GetConfig(cwd string) (Config, error)
}

type DefaultSet string

const (
	DefaultSetAll   DefaultSet = "all"
	DefaultSetNone  DefaultSet = "none"
	DefaultSetBasic DefaultSet = "basic"

	DefaultDefaultSet = DefaultSetBasic
)

var DefaultSetToRules = map[DefaultSet]RuleSet{
	DefaultSetAll:  AllRules,
	DefaultSetNone: {},
	DefaultSetBasic: func() RuleSet {
		return RuleSet{}.Add(
			PkgDocRule,
			SinglePkgDocRule,
			StartWithNameRule,
			DeprecatedRule,
		)
	}(),
}

var DefaultSetValues = func() []DefaultSet {
	values := slices.Collect(maps.Keys(DefaultSetToRules))
	slices.Sort(values)
	return values
}()

// ConfigOverride represents a configuration override.
//
// Non-nil values (including empty slices) indicate that the corresponding field
// is overridden.
type ConfigOverride struct {
	// ConfigFilePath is the path to config file.
	ConfigFilePath *string

	// Include is the overridden list of regexp patterns matching the files that
	// the linter should include.
	Include []*regexp.Regexp

	// Exclude is the overridden list of regexp patterns matching the files that
	// the linter should exclude.
	Exclude []*regexp.Regexp

	// Default is the default set of rules to enable.
	Default *DefaultSet

	// Enable is the overridden list of rules to enable.
	Enable *RuleSet

	// Disable is the overridden list of rules to disable.
	Disable *RuleSet
}

// NewConfigOverride returns a new config override instance.
func NewConfigOverride() *ConfigOverride {
	return &ConfigOverride{}
}

// Config defines an analyzer configuration.
type Config interface {
	// GetCWD returns the directory that the configuration is applied to. This
	// is the base to compute relative paths to include/exclude files.
	GetCWD() string

	// GetConfigFilePath returns the path to the configuration file. If there is
	// no configuration file, which is the case when the default is used, this
	// will be an empty string.
	GetConfigFilePath() string

	// IsAnyRuleEnabled determines if any of the given rule names is among
	// enabled rules, or not among disabled rules.
	IsAnyRuleApplicable(RuleSet) bool

	// IsPathApplicable determines if the given path matches the included path
	// patterns, or does not match the excluded path patterns.
	IsPathApplicable(path string) bool

	// Returns the rule-specific options.
	//
	// It never returns a nil pointer.
	GetRuleOptions() *RuleOptions
}

// RuleOptions represents individual linter rule configurations.
type RuleOptions struct {
	MaxLenLength                   uint `option:"max-len/length"`
	MaxLenIncludeTests             bool `option:"max-len/include-tests"`
	PkgDocIncludeTests             bool `option:"pkg-doc/include-tests"`
	SinglePkgDocIncludeTests       bool `option:"single-pkg-doc/include-tests"`
	RequirePkgDocIncludeTests      bool `option:"require-pkg-doc/include-tests"`
	RequireDocIncludeTests         bool `option:"require-doc/include-tests"`
	RequireDocIgnoreExported       bool `option:"require-doc/ignore-exported"`
	RequireDocIgnoreUnexported     bool `option:"require-doc/ignore-unexported"`
	StartWithNameIncludeTests      bool `option:"start-with-name/include-tests"`
	StartWithNameIncludeUnexported bool `option:"start-with-name/include-unexported"`
	NoUnusedLinkIncludeTests       bool `option:"no-unused-link/include-tests"`
}

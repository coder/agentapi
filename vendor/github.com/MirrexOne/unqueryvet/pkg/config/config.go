// Package config provides configuration structures for Unqueryvet analyzer.
package config

// UnqueryvetSettings holds the configuration for the Unqueryvet analyzer.
type UnqueryvetSettings struct {
	// CheckSQLBuilders enables checking SQL builders like Squirrel for SELECT * usage
	CheckSQLBuilders bool `mapstructure:"check-sql-builders" json:"check-sql-builders" yaml:"check-sql-builders"`

	// AllowedPatterns is a list of regex patterns that are allowed to use SELECT *
	// Example: ["SELECT \\* FROM temp_.*", "SELECT \\* FROM .*_backup"]
	AllowedPatterns []string `mapstructure:"allowed-patterns" json:"allowed-patterns" yaml:"allowed-patterns"`
}

// DefaultSettings returns the default configuration for unqueryvet
func DefaultSettings() UnqueryvetSettings {
	return UnqueryvetSettings{
		CheckSQLBuilders: true,
		AllowedPatterns: []string{
			`(?i)COUNT\(\s*\*\s*\)`,
			`(?i)MAX\(\s*\*\s*\)`,
			`(?i)MIN\(\s*\*\s*\)`,
			`(?i)SELECT \* FROM information_schema\..*`,
			`(?i)SELECT \* FROM pg_catalog\..*`,
			`(?i)SELECT \* FROM sys\..*`,
		},
	}
}

package config

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"

	"github.com/godoc-lint/godoc-lint/pkg/model"
)

// PlainConfig represents the plain configuration type as users would provide
// via a config file (e.g., a YAML file).
type PlainConfig struct {
	Version *string           `yaml:"version" mapstructure:"version"`
	Exclude []string          `yaml:"exclude" mapstructure:"exclude"`
	Include []string          `yaml:"include" mapstructure:"include"`
	Default *string           `yaml:"default" mapstructure:"default"`
	Enable  []string          `yaml:"enable" mapstructure:"enable"`
	Disable []string          `yaml:"disable" mapstructure:"disable"`
	Options *PlainRuleOptions `yaml:"options" mapstructure:"options"`
}

type PlainRuleOptions struct {
	MaxLenLength                   *uint `option:"max-len/length" yaml:"max-len/length" mapstructure:"max-len/length"`
	MaxLenIncludeTests             *bool `option:"max-len/include-tests" yaml:"max-len/include-tests" mapstructure:"max-len/include-tests"`
	PkgDocIncludeTests             *bool `option:"pkg-doc/include-tests" yaml:"pkg-doc/include-tests" mapstructure:"pkg-doc/include-tests"`
	SinglePkgDocIncludeTests       *bool `option:"single-pkg-doc/include-tests" yaml:"single-pkg-doc/include-tests" mapstructure:"single-pkg-doc/include-tests"`
	RequirePkgDocIncludeTests      *bool `option:"require-pkg-doc/include-tests" yaml:"require-pkg-doc/include-tests" mapstructure:"require-pkg-doc/include-tests"`
	RequireDocIncludeTests         *bool `option:"require-doc/include-tests" yaml:"require-doc/include-tests" mapstructure:"require-doc/include-tests"`
	RequireDocIgnoreExported       *bool `option:"require-doc/ignore-exported" yaml:"require-doc/ignore-exported" mapstructure:"require-doc/ignore-exported"`
	RequireDocIgnoreUnexported     *bool `option:"require-doc/ignore-unexported" yaml:"require-doc/ignore-unexported" mapstructure:"require-doc/ignore-unexported"`
	StartWithNameIncludeTests      *bool `option:"start-with-name/include-tests" yaml:"start-with-name/include-tests" mapstructure:"start-with-name/include-tests"`
	StartWithNameIncludeUnexported *bool `option:"start-with-name/include-unexported" yaml:"start-with-name/include-unexported" mapstructure:"start-with-name/include-unexported"`
	NoUnusedLinkIncludeTests       *bool `option:"no-unused-link/include-tests" yaml:"no-unused-link/include-tests" mapstructure:"no-unused-link/include-tests"`
}

func transferOptions(target *model.RuleOptions, source *PlainRuleOptions) {
	resV := reflect.ValueOf(target).Elem()
	resVT := resV.Type()

	resOptionMap := make(map[string]string, resVT.NumField())
	for i := 0; i < resVT.NumField(); i++ {
		ft := resVT.Field(i)
		key, ok := ft.Tag.Lookup("option")
		if !ok {
			continue
		}
		resOptionMap[key] = ft.Name
	}

	v := reflect.ValueOf(source).Elem()
	vt := v.Type()
	for i := 0; i < vt.NumField(); i++ {
		ft := vt.Field(i)
		key, ok := ft.Tag.Lookup("option")
		if !ok {
			continue
		}
		if ft.Type.Kind() != reflect.Pointer {
			continue
		}
		f := v.Field(i)
		if f.IsNil() {
			continue
		}
		resFieldName, ok := resOptionMap[key]
		if !ok {
			continue
		}
		resV.FieldByName(resFieldName).Set(f.Elem())
	}
}

func (pcfg *PlainConfig) Validate() error {
	var errs []error

	if pcfg.Default != nil && !slices.Contains(model.DefaultSetValues, model.DefaultSet(*pcfg.Default)) {
		errs = append(errs, fmt.Errorf("invalid default set %q; must be one of %q", *pcfg.Default, model.DefaultSetValues))
	}

	if invalids := getInvalidRules(pcfg.Enable); len(invalids) > 0 {
		errs = append(errs, fmt.Errorf("invalid rule name(s) to enable: %q", invalids))
	}

	if invalids := getInvalidRules(pcfg.Disable); len(invalids) > 0 {
		errs = append(errs, fmt.Errorf("invalid rule name(s) to disable: %q", invalids))
	}

	// To avoid being too strict, we don't complain if a rule is enabled and disabled at the same time.

	if invalids := getInvalidRegexps(pcfg.Include); len(invalids) > 0 {
		errs = append(errs, fmt.Errorf("invalid inclusion pattern(s): %q", invalids))
	}

	if invalids := getInvalidRegexps(pcfg.Exclude); len(invalids) > 0 {
		errs = append(errs, fmt.Errorf("invalid exclusion pattern(s): %q", invalids))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func getInvalidRules(names []string) []string {
	invalids := make([]string, 0, len(names))
	for _, element := range names {
		if !model.AllRules.Has(model.Rule(element)) {
			invalids = append(invalids, element)
		}
	}
	return invalids
}

func getInvalidRegexps(values []string) []string {
	invalids := make([]string, 0, len(values))
	for _, element := range values {
		if _, err := regexp.Compile(element); err != nil {
			invalids = append(invalids, element)
		}
	}
	return invalids
}

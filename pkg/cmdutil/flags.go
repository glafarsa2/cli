package cmdutil

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// NilStringFlag defines a new flag with a string pointer receiver. This is useful for differentiating
// between the flag being set to a blank value and the flag not being passed at all.
func NilStringFlag(cmd *cobra.Command, p **string, name string, shorthand string, usage string) *pflag.Flag {
	return cmd.Flags().VarPF(newStringValue(p), name, shorthand, usage)
}

// NilBoolFlag defines a new flag with a bool pointer receiver. This is useful for differentiating
// between the flag being explicitly set to a false value and the flag not being passed at all.
func NilBoolFlag(cmd *cobra.Command, p **bool, name string, shorthand string, usage string) *pflag.Flag {
	f := cmd.Flags().VarPF(newBoolValue(p), name, shorthand, usage)
	f.NoOptDefVal = "true"
	return f
}

// StringEnumFlag defines a new string flag that only allows values listed in options.
func StringEnumFlag(cmd *cobra.Command, p *string, name, shorthand, defaultValue string, options []string, usage string) *pflag.Flag {
	val := &enumValue{string: p, options: options}
	f := cmd.Flags().VarPF(val, name, shorthand, fmt.Sprintf("%s: %s", usage, formatValuesForUsageDocs(options)))
	_ = cmd.RegisterFlagCompletionFunc(name, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return options, cobra.ShellCompDirectiveNoFileComp
	})
	return f
}

func formatValuesForUsageDocs(values []string) string {
	return fmt.Sprintf("{%s}", strings.Join(values, "|"))
}

type stringValue struct {
	string **string
}

func newStringValue(p **string) *stringValue {
	return &stringValue{p}
}

func (s *stringValue) Set(value string) error {
	*s.string = &value
	return nil
}

func (s *stringValue) String() string {
	if s.string == nil || *s.string == nil {
		return ""
	}
	return **s.string
}

func (s *stringValue) Type() string {
	return "string"
}

type boolValue struct {
	bool **bool
}

func newBoolValue(p **bool) *boolValue {
	return &boolValue{p}
}

func (b *boolValue) Set(value string) error {
	v, err := strconv.ParseBool(value)
	*b.bool = &v
	return err
}

func (b *boolValue) String() string {
	if b.bool == nil || *b.bool == nil {
		return "false"
	} else if **b.bool {
		return "true"
	}
	return "false"
}

func (b *boolValue) Type() string {
	return "bool"
}

func (b *boolValue) IsBoolFlag() bool {
	return true
}

type enumValue struct {
	string  *string
	options []string
}

func (e *enumValue) Set(value string) error {
	found := false
	for _, opt := range e.options {
		if strings.EqualFold(opt, value) {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("valid values are %s", formatValuesForUsageDocs(e.options))
	}
	*e.string = value
	return nil
}

func (e *enumValue) String() string {
	return *e.string
}

func (e *enumValue) Type() string {
	return "string"
}

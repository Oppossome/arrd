package httpapi

import (
	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/httpapi/apigen"
)

func toAPISettingField(f formspec.Field) apigen.SettingField {
	return apigen.SettingField{
		Key:         f.Key,
		Label:       f.Label,
		Kind:        apigen.SettingFieldKind(f.Kind),
		Required:    optional(f.Required),
		Help:        optional(f.Help),
		Placeholder: optional(f.Placeholder),
		Default:     optional(f.Default),
		Secret:      optional(f.Secret),
		Options:     optional(f.Options),
		Choices:     optSlice(f.Choices, toAPISettingOption),
		FileAccept:  optional(f.FileAccept),
	}
}

func toAPISettingOption(o formspec.Option) apigen.SettingOption {
	return apigen.SettingOption{Value: o.Value, Label: o.Label, Tags: optSlice(o.Tags, identity[string])}
}

package autobrr

import (
	"strings"

	"github.com/nerney/ptv/internal/autobrrdefs"
	sharedsettings "github.com/nerney/ptv/internal/settings"
)

const ExistingSecretValue = sharedsettings.ExistingSecretValue

// SettingsFromPairs converts Autobrr's GET response setting slice into the
// persisted map shape used by PTV.
func SettingsFromPairs(in []Setting) map[string]string {
	out := make(map[string]string, len(in))
	for _, s := range in {
		out[s.Name] = s.Value
	}
	return out
}

// MergeSettings applies submitted values to existing Autobrr settings using
// the checked-in Autobrr definition as the field contract. Unknown keys are
// dropped only when a valid definition is available to define that contract.
func MergeSettings(def autobrrdefs.Def, existing, submitted map[string]string) map[string]string {
	return sharedsettings.Merge(contractSettingsFields(def), existing, submitted)
}

// WithCoreCredentials overlays PTV's core tracker credential onto Autobrr
// fields that are known credential slots in the Autobrr definition. The
// tracker URL remains the indexer root base_url payload field.
func WithCoreCredentials(def autobrrdefs.Def, settings map[string]string, apiKey string) map[string]string {
	out := MergeSettings(def, settings, nil)
	for _, f := range defFields(def) {
		if isCredentialField(f.Name) {
			out[f.Name] = apiKey
		}
	}
	return out
}

func defFields(def autobrrdefs.Def) []autobrrdefs.Setting {
	out := make([]autobrrdefs.Setting, 0, len(def.Settings)+len(def.IRCSettings))
	seen := make(map[string]bool, len(def.Settings)+len(def.IRCSettings))
	for _, group := range [][]autobrrdefs.Setting{def.Settings, def.IRCSettings} {
		for _, f := range group {
			key := strings.ToLower(strings.TrimSpace(f.Name))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, f)
		}
	}
	return out
}

func isSecretDefField(f autobrrdefs.Setting) bool {
	return strings.EqualFold(f.Type, "secret") || isCredentialField(f.Name)
}

func settingsFields(def autobrrdefs.Def) []sharedsettings.Field {
	root := settingsFieldsForLayer(def.Settings, "root")
	irc := settingsFieldsForLayer(def.IRCSettings, "irc")
	return append(root, irc...)
}

func contractSettingsFields(def autobrrdefs.Def) []sharedsettings.Field {
	return settingsFieldsForLayer(defFields(def), "")
}

func settingsFieldsForLayer(fields []autobrrdefs.Setting, layer string) []sharedsettings.Field {
	out := make([]sharedsettings.Field, 0, len(fields))
	for _, f := range fields {
		out = append(out, sharedsettings.Field{
			Name:       f.Name,
			Label:      f.Label,
			HelpText:   f.Help,
			Type:       f.Type,
			Default:    f.Default,
			HasDefault: f.Default != "",
			Secret:     isSecretDefField(f),
			Required:   f.Required,
			Layer:      layer,
			URL:        sharedsettings.IsURLName(f.Name),
			SkipDrift:  isSecretDefField(f) || isCredentialField(f.Name),
		})
	}
	return out
}

// SettingField is the sanitized view of an Autobrr schema field that the UI
// may render. Secret values are represented by ExistingSecretValue, never by
// the stored secret itself.
type SettingField struct {
	Name     string
	Label    string
	Help     string
	Type     string
	Value    string
	HasValue bool
	Secret   bool
	Required bool
	Layer    string // "root" or "irc"
}

// RenderFields returns all definition fields with values safe for frontend use.
// Settings are grouped by layer (root vs IRC) for template rendering.
func RenderFields(def autobrrdefs.Def, settings map[string]string) []SettingField {
	rendered := sharedsettings.Render(settingsFields(def), settings)
	out := make([]SettingField, 0, len(rendered))
	for _, r := range rendered {
		out = append(out, SettingField{
			Name:     r.Name,
			Label:    r.Label,
			Help:     r.HelpText,
			Type:     r.Type,
			Value:    r.Value,
			HasValue: r.HasValue,
			Secret:   r.Secret,
			Required: r.Required,
			Layer:    r.Layer,
		})
	}
	return out
}

// DiffSettings returns definition field names whose normalized values differ.
func DiffSettings(def autobrrdefs.Def, desired, actual map[string]string) []string {
	return sharedsettings.Diff(contractSettingsFields(def), desired, actual)
}

package settings

import (
	"sort"
	"strconv"
	"strings"
)

const ExistingSecretValue = "__ptv_existing_secret__"

type Field struct {
	Name          string
	Label         string
	HelpText      string
	HelpLink      string
	Placeholder   string
	Type          string
	Default       string
	HasDefault    bool
	Secret        bool
	Info          bool
	Required      bool
	Advanced      bool
	Layer         string
	URL           bool
	Number        bool
	Bool          bool
	SkipRender    bool
	SkipDrift     bool
	SelectOptions []Option
}

type Option struct {
	Name  string
	Value string
	Hint  string
}

type RenderedField struct {
	Name          string
	Label         string
	HelpText      string
	HelpLink      string
	Placeholder   string
	Type          string
	Value         string
	HasValue      bool
	Secret        bool
	Info          bool
	Required      bool
	Advanced      bool
	Layer         string
	SelectOptions []Option
}

func Merge(fields []Field, existing, submitted map[string]string) map[string]string {
	out := make(map[string]string, len(fields))
	for _, f := range fields {
		current, hasCurrent := existing[f.Name]
		if !hasCurrent && f.HasDefault {
			current = f.Default
			hasCurrent = true
		}
		next, submittedField := submitted[f.Name]
		if submittedField {
			if f.Secret && (next == "" || next == ExistingSecretValue) && hasCurrent {
				out[f.Name] = current
				continue
			}
			out[f.Name] = next
			continue
		}
		if hasCurrent {
			out[f.Name] = current
		}
	}
	return out
}

func Render(fields []Field, values map[string]string) []RenderedField {
	out := make([]RenderedField, 0, len(fields))
	for _, f := range fields {
		if f.SkipRender {
			continue
		}
		v, ok := values[f.Name]
		if !ok && f.HasDefault {
			v = f.Default
			ok = true
		}
		r := RenderedField{
			Name:          f.Name,
			Label:         f.Label,
			HelpText:      f.HelpText,
			HelpLink:      f.HelpLink,
			Placeholder:   f.Placeholder,
			Type:          f.Type,
			HasValue:      ok && v != "",
			Secret:        f.Secret,
			Info:          f.Info,
			Required:      f.Required,
			Advanced:      f.Advanced,
			Layer:         f.Layer,
			SelectOptions: append([]Option(nil), f.SelectOptions...),
		}
		if f.Info {
			if r.HelpText == "" {
				r.HelpText = v
			}
		} else if !f.Secret {
			r.Value = v
		} else if r.HasValue {
			r.Value = ExistingSecretValue
		}
		out = append(out, r)
	}
	return out
}

func Diff(fields []Field, left, right map[string]string) []string {
	l := Merge(fields, left, nil)
	r := Merge(fields, right, nil)
	var diff []string
	for _, f := range fields {
		if f.SkipDrift {
			continue
		}
		lv, lok := l[f.Name]
		rv, rok := r[f.Name]
		if comparableValue(f, lv, lok) != comparableValue(f, rv, rok) {
			diff = append(diff, f.Name)
		}
	}
	sort.Strings(diff)
	return diff
}

func Equal(fields []Field, left, right map[string]string) bool {
	return len(Diff(fields, left, right)) == 0
}

func comparableValue(f Field, value string, ok bool) string {
	if !ok {
		value = ""
		if f.HasDefault {
			value = f.Default
		}
	}
	if f.Bool || strings.EqualFold(f.Type, "checkbox") || strings.EqualFold(f.Type, "bool") {
		if value == "" || strings.EqualFold(value, "false") || value == "0" {
			return "false"
		}
		return strconv.FormatBool(value == "true" || value == "on" || value == "1")
	}
	if f.URL {
		return NormalizeURL(value)
	}
	if f.Number || strings.EqualFold(f.Type, "number") {
		if n, err := strconv.ParseFloat(value, 64); err == nil {
			return strconv.FormatFloat(n, 'f', -1, 64)
		}
	}
	return value
}

func IsCredentialName(name string) bool {
	low := strings.ToLower(name)
	return low == "apikey" ||
		low == "api_key" ||
		low == "passkey" ||
		low == "apitoken" ||
		strings.Contains(low, "key") ||
		strings.Contains(low, "token")
}

func IsURLName(name string) bool {
	low := strings.ToLower(name)
	return low == "baseurl" || low == "sitelink" || strings.Contains(low, "url")
}

func NormalizeURL(value string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(value)), "/")
}

package autobrrdefs

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Def is a parsed autobrr indexer definition — the fields PTV needs to
// drive the configuration UI and populate the autobrr API.
type Def struct {
	Name        string
	Identifier  string
	Description string
	URLs        []string
	Privacy     string
	Supports    []string // "irc", "rss", etc.

	// Settings are the two layers of user-configurable fields:
	//   Settings    — top-level (RSS key, API key, base URL, etc.)
	//   IRCSettings — under irc.settings (nick, passkey, invite key, etc.)
	Settings    []Setting
	IRCSettings []Setting

	// IRC connection metadata — informational, not user-editable.
	IRCNetwork  string
	IRCServer   string
	IRCPort     int
	IRCChannels []string
	Announcers  []string
}

// Setting is one user-configurable field from a definition file.
type Setting struct {
	Name     string
	Type     string // "text", "secret", "number", etc.
	Required bool
	Label    string
	Help     string
	Default  string
}

// defFile is the subset of an autobrr YAML definition that PTV reads.
// The parse/regex sections are intentionally omitted — they're only
// needed by autobrr itself, not by PTV's config UI.
type defFile struct {
	Name        string        `yaml:"name"`
	Identifier  string        `yaml:"identifier"`
	Description string        `yaml:"description"`
	URLs        []string      `yaml:"urls"`
	Privacy     string        `yaml:"privacy"`
	Supports    []string      `yaml:"supports"`
	Settings    []settingYAML `yaml:"settings"`
	IRC         *ircYAML      `yaml:"irc"`
}

type settingYAML struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Label    string `yaml:"label"`
	Help     string `yaml:"help"`
	Default  string `yaml:"default"`
}

type ircYAML struct {
	Network    string        `yaml:"network"`
	Server     string        `yaml:"server"`
	Port       int           `yaml:"port"`
	TLS        bool          `yaml:"tls"`
	Channels   []string      `yaml:"channels"`
	Announcers []string      `yaml:"announcers"`
	Settings   []settingYAML `yaml:"settings"`
}

// parseCatalog reads every *.yaml file under dir and returns one Def per
// parseable file. Files that fail to parse are silently skipped.
func parseCatalog(dir string) ([]Def, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	var out []Def
	for _, f := range files {
		if d := parseFile(f); d != nil {
			out = append(out, *d)
		}
	}
	return out, nil
}

func parseFile(path string) *Def {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var f defFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil
	}
	if f.Name == "" || f.Identifier == "" {
		return nil
	}
	d := &Def{
		Name:        f.Name,
		Identifier:  f.Identifier,
		Description: f.Description,
		URLs:        f.URLs,
		Privacy:     f.Privacy,
		Supports:    f.Supports,
		Settings:    toSettings(f.Settings),
	}
	if f.IRC != nil {
		d.IRCSettings = toSettings(f.IRC.Settings)
		d.IRCNetwork  = f.IRC.Network
		d.IRCServer   = f.IRC.Server
		d.IRCPort     = f.IRC.Port
		d.IRCChannels = f.IRC.Channels
		d.Announcers  = f.IRC.Announcers
	}
	return d
}

func toSettings(in []settingYAML) []Setting {
	if len(in) == 0 {
		return nil
	}
	out := make([]Setting, len(in))
	for i, s := range in {
		out[i] = Setting{
			Name:     s.Name,
			Type:     s.Type,
			Required: s.Required,
			Label:    s.Label,
			Help:     s.Help,
			Default:  s.Default,
		}
	}
	return out
}

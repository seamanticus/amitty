package config

import (
	"bytes"
	"encoding"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// OptionKind is how an option is edited.
type OptionKind uint8

const (
	// OptionText is edited as a TOML value typed in full, such as `"ctrl+k"` or `{ attributes = "bold" }`.
	OptionText OptionKind = iota
	// OptionBool is toggled.
	OptionBool
	// OptionChoice is picked from Option.Choices.
	OptionChoice
	// OptionFileOnly is a table of entries of the user's own, such as openers, edited in the file.
	OptionFileOnly
)

// Option is one option of the config: its dotted TOML path, its documentation, and its value as TOML.
type Option struct {
	Path, Doc, Value string
	Kind             OptionKind
	// Choices are the values an OptionChoice takes, each as TOML.
	Choices []string
	// Restart reports whether a change applies only after amitty restarts, as it is used once at startup.
	Restart bool
}

// restartOptions are used once at startup: the connection, the token store, and what the screen is set up with.
func restartOptions() []string {
	return []string{"proxy", "token-encrypt-cmd", "token-decrypt-cmd", "mouse", "notifications.when_unfocused", "ui.messages_list.image_protocol"}
}

// optionChoices returns the values an option is picked from, as TOML, or nil if it is typed.
func optionChoices(path string) []string {
	quote := func(values ...string) []string {
		quoted := make([]string, len(values))
		for i, v := range values {
			quoted[i] = fmt.Sprintf("%q", v)
		}
		return quoted
	}
	switch path {
	case "theme_profile":
		return quote(append([]string{""}, ProfileNames()...)...)
	case "status":
		return quote("default", "online", "dnd", "idle", "invisible", "offline")
	case "ui.messages_list.image_protocol":
		return quote("auto", "kitty", "sixel", "halfblocks")
	case "ui.messages_list.avatars.shape":
		return quote("square", "rounded", "circle")
	case "theme.messages_list.author_color":
		return quote("role", "style")
	}
	return nil
}

// Options returns every option of cfg, in the order the config file lists them.
func Options(cfg *Config) []Option {
	docs := optionDocs()
	var options []Option
	var walk func(path string, table reflect.Value)
	walk = func(path string, table reflect.Value) {
		for _, f := range tomlFields(table) {
			name := f.key
			if path != "" {
				name = path + "." + f.key
			}
			if isTable(f.value) {
				walk(name, f.value)
				continue
			}
			option := Option{Path: name, Doc: docs[name], Value: encodeValue(f.key, f.value), Restart: slices.Contains(restartOptions(), name)}
			switch {
			case f.value.Kind() == reflect.Bool:
				option.Kind = OptionBool
			case f.value.Kind() == reflect.Map:
				option.Kind = OptionFileOnly
			case optionChoices(name) != nil:
				option.Kind, option.Choices = OptionChoice, optionChoices(name)
			}
			options = append(options, option)
		}
	}
	walk("", reflect.ValueOf(cfg).Elem())
	return options
}

type tomlField struct {
	key   string
	value reflect.Value
}

// tomlFields returns the fields of a struct value by TOML key, with those of embedded structs in place.
func tomlFields(value reflect.Value) []tomlField {
	var fields []tomlField
	for i := range value.NumField() {
		f := value.Type().Field(i)
		if f.Anonymous {
			fields = append(fields, tomlFields(value.Field(i))...)
			continue
		}
		if key := f.Tag.Get("toml"); key != "" {
			fields = append(fields, tomlField{key, value.Field(i)})
		}
	}
	return fields
}

// isTable reports whether value is written as a TOML table rather than as a value.
func isTable(value reflect.Value) bool {
	switch value.Interface().(type) {
	case toml.Marshaler, encoding.TextMarshaler:
		return false
	}
	return value.Kind() == reflect.Struct
}

// encodeValue returns value as the TOML that follows `key = `. A map is summarized, as TOML writes it as a table of its own.
func encodeValue(key string, value reflect.Value) string {
	if value.Kind() == reflect.Map {
		if value.Len() == 0 {
			return "{}"
		}
		return fmt.Sprintf("{ %d entries }", value.Len())
	}
	var line bytes.Buffer
	if err := toml.NewEncoder(&line).Encode(map[string]any{key: value.Interface()}); err != nil {
		return "?"
	}
	_, v, _ := strings.Cut(line.String(), " = ")
	return strings.TrimSpace(v)
}

// SetOption returns data, a config file, with the option at path set to value, a TOML value, changing as little of the file as it can: the option's own line, else its commented-out default, else a new line in its table.
// It fails if value is not valid for the option, or if the file does not then give the option that value.
func SetOption(data []byte, path, value string) ([]byte, error) {
	table, key := "", path
	if i := strings.LastIndex(path, "."); i >= 0 {
		table, key = path[:i], path[i+1:]
	}
	line := key + " = " + value
	var probe map[string]any
	if _, err := toml.Decode(line, &probe); err != nil {
		return nil, fmt.Errorf("%s: not a TOML value: %w", value, err)
	}

	edited := placeOption(strings.SplitAfter(string(data), "\n"), table, key, line)
	cfg, err := decode([]byte(edited))
	if err != nil {
		return nil, err
	}
	// The value must read back as written, or the line landed where TOML does not take it as this option.
	want, err := normalizedValue(path, line)
	if err != nil {
		return nil, err
	}
	for _, option := range Options(&cfg) {
		if option.Path == path {
			if option.Value != want {
				return nil, fmt.Errorf("%s reads back as %s, not %s: edit the file by hand", path, option.Value, want)
			}
			return []byte(edited), nil
		}
	}
	return nil, fmt.Errorf("no option %s", path)
}

// normalizedValue returns the value of line, `key = value`, decoded as the option at path and encoded again, as Options shows it.
func normalizedValue(path, line string) (string, error) {
	defaults := Default()
	field, ok := fieldAt(reflect.ValueOf(&defaults).Elem(), path)
	if !ok {
		return "", fmt.Errorf("no option %s", path)
	}
	key := path[strings.LastIndex(path, ".")+1:]
	holder := reflect.New(reflect.StructOf([]reflect.StructField{{Name: "V", Type: field.Type(), Tag: reflect.StructTag(`toml:"` + key + `"`)}}))
	if _, err := toml.Decode(line, holder.Interface()); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return encodeValue(key, holder.Elem().Field(0)), nil
}

// fieldAt returns the field of table at the dotted TOML path.
func fieldAt(table reflect.Value, path string) (reflect.Value, bool) {
	head, rest, nested := strings.Cut(path, ".")
	for _, f := range tomlFields(table) {
		if f.key != head {
			continue
		}
		if !nested {
			return f.value, true
		}
		return fieldAt(f.value, rest)
	}
	return reflect.Value{}, false
}

// placeOption returns lines with line set as key in table: over the key's line, over its commented-out line, after the table's header, or in a new table.
func placeOption(lines []string, table, key, line string) string {
	isKey := func(text, prefix string) bool {
		rest, ok := strings.CutPrefix(strings.TrimSpace(text), prefix)
		if !ok {
			return false
		}
		rest, ok = strings.CutPrefix(strings.TrimLeft(rest, " \t"), key)
		return ok && strings.HasPrefix(strings.TrimLeft(rest, " \t"), "=")
	}

	current, header, firstHeader, commented := "", -1, -1, -1
	for i, text := range lines {
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "[[") {
			current = strings.TrimSpace(strings.Trim(trimmed, "[]"))
			if firstHeader < 0 {
				firstHeader = i
			}
			if current == table {
				header = i
			}
			continue
		}
		if current != table {
			continue
		}
		if isKey(text, "") {
			return strings.Join(slices.Concat(lines[:i], []string{line + "\n"}, lines[valueEnd(lines, i):]), "")
		}
		if commented < 0 && isKey(text, "#") {
			commented = i
		}
	}

	switch {
	case commented >= 0:
		return strings.Join(slices.Concat(lines[:commented], []string{line + "\n"}, lines[commented+1:]), "")
	case table == "" && firstHeader >= 0:
		// Top-level keys must come before the first table.
		return strings.Join(slices.Concat(lines[:firstHeader], []string{line + "\n"}, lines[firstHeader:]), "")
	case table == "" || header >= 0:
		at := len(lines)
		if header >= 0 {
			at = header + 1
		}
		return strings.Join(slices.Concat(lines[:at], []string{line + "\n"}, lines[at:]), "")
	}
	text := strings.Join(lines, "")
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text + "\n[" + table + "]\n" + line + "\n"
}

// valueEnd returns the index after the last line of the value starting at lines[start], which may span lines as an array or a multi-line string does.
func valueEnd(lines []string, start int) int {
	for end := start + 1; end <= len(lines); end++ {
		var probe map[string]any
		if _, err := toml.Decode(strings.Join(lines[start:end], ""), &probe); err == nil {
			return end
		}
	}
	return start + 1
}

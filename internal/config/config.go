// Package config loads the optional sopro.conf file (KEY=VALUE lines).
// It sits below flags and environment variables: an explicit flag wins,
// then SOPRO_* environment, then the config file, then built-in defaults.
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// File holds parsed KEY=VALUE pairs.
type File map[string]string

// DefaultPath returns the conventional config location.
func DefaultPath() string {
	directory, err := os.UserConfigDir()
	if err != nil || directory == "" {
		return "sopro.conf"
	}
	return filepath.Join(directory, "sopro", "sopro.conf")
}

// Path returns the SOPRO_CONFIG override when set, else DefaultPath.
func Path() string {
	if value := strings.TrimSpace(os.Getenv("SOPRO_CONFIG")); value != "" {
		return value
	}
	return DefaultPath()
}

// Load parses path, returning an empty File when it does not exist. Lines
// starting with '#' and blank lines are ignored; surrounding single or
// double quotes are stripped from values.
func Load(path string) (File, error) {
	file := File{}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return file, nil
		}
		return nil, err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(stripQuotes(strings.TrimSpace(value)))
		if key == "" {
			continue
		}
		file[key] = value
	}
	return file, scanner.Err()
}

// Lookup returns the file value for key.
func (f File) Lookup(key string) (string, bool) {
	if f == nil {
		return "", false
	}
	value, ok := f[key]
	return value, ok
}

func stripQuotes(value string) string {
	if len(value) >= 2 {
		if first, last := value[0], value[len(value)-1]; first == last && (first == '"' || first == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

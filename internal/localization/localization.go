// Package localization provides functionality for internationalization (i18n).
// It loads translation strings from JSON files and provides a simple way to get
// localized strings for different languages.
package localization

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Localizer manages the translations for the application.
// It holds a map of languages, each with its own map of translation keys and values.
type Localizer struct {
	translations map[string]map[string]string
	mu           sync.RWMutex
}

// NewLocalizer creates and returns a new Localizer instance.
// It loads all translations from the provided directory path.
// The directory should contain JSON files named with the language code (e.g., "en.json").
func NewLocalizer(path string) (*Localizer, error) {
	l := &Localizer{
		translations: make(map[string]map[string]string),
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read localization directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		lang := strings.TrimSuffix(file.Name(), ".json")
		filePath := filepath.Join(path, file.Name())

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read localization file %s: %w", file.Name(), err)
		}

		var translations map[string]string
		if err := json.Unmarshal(data, &translations); err != nil {
			return nil, fmt.Errorf("failed to parse localization file %s: %w", file.Name(), err)
		}

		l.translations[lang] = translations
	}

	return l, nil
}

// GetString returns the localized string for a given key and language.
// If the language or the key is not found, it returns the key itself as a fallback.
func (l *Localizer) GetString(lang, key string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if langTranslations, ok := l.translations[lang]; ok {
		if value, ok := langTranslations[key]; ok {
			return value
		}
	}

	// Fallback to a default language if the key is not found in the specified language
	if lang != "en" {
		if enTranslations, ok := l.translations["en"]; ok {
			if value, ok := enTranslations[key]; ok {
				return value
			}
		}
	}

	return key
}

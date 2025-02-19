package language

import (
	"fmt"
	"os"
	"strings"
)

var ErrorLanguageNotAvailable = fmt.Errorf("Language not available")

type LanguagePack struct {
	Languages []string
}

func BuildLanguagePack() (*LanguagePack, error) {
	l := &LanguagePack{}
	err := l.LoadLanguages()
	if err != nil {
		return nil, err
	}

	return l, nil
}

func (l *LanguagePack) LoadLanguages() error {
	l.Languages = []string{}
	entries, err := os.ReadDir("localization")
	if err != nil {
		return ErrorLanguageNotAvailable

	}

	for _, e := range entries {
		parts := strings.Split(e.Name(), ".")
		if len(parts) > 1 {
			l.Languages = append(l.Languages, parts[1])
		}
	}

	return nil
}

func (l LanguagePack) HasLanguage(language string) bool {
	for _, lang := range l.Languages {
		if lang == language {
			return true
		}
	}

	return false
}

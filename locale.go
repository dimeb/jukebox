package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/leonelquinteros/gotext"
)

// Locale structure.
type Locale struct {
	*gotext.Locale
}

var locale *Locale

// NewLocale intializes Locale.
func NewLocale() (*Locale, error) {
	l := &Locale{gotext.NewLocale(`templates`, cfg.WebAdminLanguage)}

	err := filepath.Walk(
		`./templates/`+cfg.WebAdminLanguage,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				f := filepath.Base(info.Name())
				if strings.HasSuffix(f, `.po`) {
					l.AddDomain(f[:len(f)-3])
				}
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return l, nil
}

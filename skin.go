package main

import (
	"fmt"
	"net/http"
	"strings"
)

// Skin structure.
type Skin struct {
	folder string
	prefix string
}

var skin *Skin

// NewSkin creates new Skin structure.
func NewSkin() *Skin {
	return &Skin{
		folder: `img`,
	}
}

func (s *Skin) updateFromWebAdmin(r *http.Request, skins []string) (msgOK, msgErr map[string][]interface{}) {
	var err error = nil
	msgOK = make(map[string][]interface{})
	msgErr = make(map[string][]interface{})
	newCf := cfg.copy()
	changed := false
	skin := strings.TrimSpace(r.FormValue(`skin`))
	if skin != cfg.Skin {
		for _, s := range skins {
			if s == skin {
				newCf.Skin = skin
				changed = true
			}
		}
	}
	if changed {
		err = newCf.save()
		if err == nil {
			cfg = newCf.copy()
			if userInterface.wsCanWrite {
				userInterface.screenMessageChannel <- `skin`
			}
			msgOK[`Configuration changed`] = nil
		} else {
			logger.queue <- fmt.Sprint(err)
			msgErr[`Error saving data please try again`] = nil
		}
	}
	return
}

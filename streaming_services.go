package main

import (
	"fmt"
	"net/http"
	"strings"
)

type StreamingServices struct {
	updateChannel chan string
}

var streamingServices *StreamingServices

// NewStreamingServices creates new Streaming structure.
func NewStreamingServices() *StreamingServices {
	return &StreamingServices{
		updateChannel: make(chan string, 10),
	}
}

func (ss *StreamingServices) update() {
	for {
		select {
		case origin := <-ss.updateChannel:
			switch origin {
			case `spotify`:
				//
			}
		}
	}
}

// Updates configuration with values from web admin page.
func (ss *StreamingServices) updateFromWebAdmin(r *http.Request) (msgOK, msgErr map[string][]interface{}) {
	var err error = nil
	msgOK = make(map[string][]interface{})
	msgErr = make(map[string][]interface{})
	newCfg := cfg.copy()
	changed := false
	for k, v := range cfg.StreamingServices {
		for v1, v2 := range v {
			val := strings.TrimSpace(r.FormValue(k + `-` + v1))
			if v2 != val {
				if v1 == `active` && val == `` {
					val = `0`
				}
				newCfg.StreamingServices[k][v1] = val
				changed = true
			}
		}
	}

	if changed {
		err = newCfg.save()
		if err == nil {
			cfg = newCfg.copy()
			msgOK[`Configuration changed`] = nil
		} else {
			logger.queue <- fmt.Sprint(err)
			msgErr[`Error saving data please try again`] = nil
		}
	}
	return
}

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// WebAdmin the Web administration structure.
type WebAdmin struct {
	action            chan byte
	token             map[string]interface{}
	templateDirectory string
	templateExtension string
	functions         template.FuncMap
	templates         []string
	path              string
	stylesheets       []string
	messageOK         map[string][]interface{}
	messageError      map[string][]interface{}
	data              interface{}
}

var webAdmin *WebAdmin

// NewWebAdmin intializes WebAdmin, starts web servers.
func NewWebAdmin() *WebAdmin {
	// Initialize WebAdmin.
	wa := &WebAdmin{
		action:            make(chan byte),
		token:             make(map[string]interface{}),
		templateDirectory: `templates/`,
		templateExtension: `.gohtml`,
		functions: template.FuncMap{
			`StringsJoin`:  strings.Join,
			`StringsSplit`: strings.Split,
			`HTMLString`: func(s string) template.HTML {
				return template.HTML(s)
			},
			`SliceToText`: func(sl []string) string {
				return strings.Join(sl, "\r\n")
			},
			`Increment`: func(i int) int {
				return i + 1
			},
			`Decrement`: func(i int) int {
				return i - 1
			},
			"Mod": func(i, j int) bool {
				return i%j == 0
			},
			"Atoi": func(s string) int {
				i, err := strconv.Atoi(s)
				if err != nil {
					logger.queue <- fmt.Sprint(err)
					return -1
				}
				return i
			},
		},
	}

	// Create router.
	mux := http.NewServeMux()
	mux.Handle(`/css/`, http.StripPrefix(`/css/`, http.FileServer(http.Dir(`css`))))
	mux.Handle(`/js/`, http.StripPrefix(`/js/`, http.FileServer(http.Dir(`js`))))
	mux.Handle(`/img/`, http.StripPrefix(`/img/`, http.FileServer(http.Dir(`img`))))
	mux.HandleFunc(`/`, wa.response)

	// HTTP/HTTP servers start/restart goroutine.
	go func() {
		var (
			c byte
			m sync.RWMutex
		)

		servers := make(map[string]*http.Server)
		for {
			select {
			case c = <-wa.action:
			}

			// Remove servers from list.
			if c == 'R' {
				for a := range servers {
					// Remove server which listens on address a.
					m.RLock()
					_, ok := servers[a]
					m.RUnlock()
					if ok {
						// Gracefully shutdown server which listens on address a.
						logger.queue <- fmt.Sprintf("shutting down HTTP/HTTPS service on %s ...", a)
						if err := servers[a].Shutdown(context.Background()); err == nil {
							logger.queue <- fmt.Sprintf("HTTP/HTTPS service on %s shut down successfully.", a)
						} else {
							logger.queue <- fmt.Sprintf("error shutting down HTTP/HTTPS service on %s: %s", a, err.Error())
						}
						m.Lock()
						delete(servers, a)
						m.Unlock()
					}
				}
			}

			for _, addr := range cfg.WebAdminHTTPAddress {
				go func(a string) {
					logger.queue <- fmt.Sprintf("Starting HTTP server on %s ...", a)
					h := &http.Server{
						Addr:         a,
						Handler:      mux,
						ReadTimeout:  5 * time.Second,
						WriteTimeout: 5 * time.Second,
						IdleTimeout:  5 * time.Second,
					}
					m.RLock()
					_, ok := servers[a]
					m.RUnlock()
					if ok {
						logger.queue <- `address already in use`
					}
					m.Lock()
					servers[a] = h
					m.Unlock()
					logger.queue <- fmt.Sprint(h.ListenAndServe())
				}(addr)
			}
			for _, addr := range cfg.WebAdminHTTPSAddress {
				go func(a string) {
					logger.queue <- fmt.Sprintf("Starting HTTPS server on %s ...", a)
					h := &http.Server{
						Addr:         a,
						Handler:      mux,
						ReadTimeout:  5 * time.Second,
						WriteTimeout: 5 * time.Second,
						IdleTimeout:  5 * time.Second,
						TLSConfig: &tls.Config{
							MinVersion:               tls.VersionTLS12,
							CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
							PreferServerCipherSuites: true,
							CipherSuites: []uint16{
								tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
								tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
								tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
								tls.TLS_RSA_WITH_AES_256_CBC_SHA,
							},
						},
						TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
					}
					m.RLock()
					_, ok := servers[a]
					m.RUnlock()
					if ok {
						logger.queue <- `address already in use`
					}
					m.Lock()
					servers[a] = h
					m.Unlock()
					logger.queue <- fmt.Sprint(h.ListenAndServe())
					logger.queue <- fmt.Sprint(h.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile))
				}(addr)
			}
		}
	}()

	return wa
}

func (wa *WebAdmin) createTokenKey(r *http.Request) (string, error) {
	return hash(r.RemoteAddr + `:` + cfg.WebAdminUsername + `:` + cfg.WebAdminPassword)
}

func (wa *WebAdmin) isTokenKeySet(key string) bool {
	_, ok := wa.token[key]
	return ok
}

func (wa *WebAdmin) setTokenKey(r *http.Request) (string, error) {
	key, err := hash(r.RemoteAddr + `:` + cfg.WebAdminUsername + `:` + cfg.WebAdminPassword)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return ``, nil
	}
	wa.token[key] = nil
	return key, nil
}

// Process HTTP/HTPS request and writes response.
func (wa *WebAdmin) response(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	cookieVal := ``

	// Initialize templates.
	wa.templates = []string{}

	// Initialize messages.
	wa.messageOK = make(map[string][]interface{})
	wa.messageError = make(map[string][]interface{})

	// Authentication check.
	c, err := r.Cookie(cfg.webAdminCookieName)
	if err != nil {
		if err == http.ErrNoCookie {
			status = http.StatusUnauthorized
			if u, p, ok := r.BasicAuth(); ok && u == cfg.WebAdminUsername {
				if err = cfg.checkPassword(p); err == nil {
					cookieVal, err = wa.setTokenKey(r)
					if err == nil {
						status = http.StatusOK
					}
				}
				if err != nil {
					logger.queue <- fmt.Sprint(err)
				}
			}
		} else {
			status = http.StatusBadRequest
		}
	} else {
		cv, err := base64.StdEncoding.DecodeString(c.Value)
		if err == nil {
			cookieVal = string(cv)
		}
		if err != nil || !wa.isTokenKeySet(cookieVal) {
			status = http.StatusUnauthorized
		}
	}

	if status != http.StatusOK {
		http.SetCookie(w, &http.Cookie{
			Name:     cfg.webAdminCookieName,
			Value:    ``,
			HttpOnly: true,
			Secure:   false,
			Path:     `/`,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
		})
		if status == http.StatusBadRequest {
			logger.queue <- fmt.Sprint(err)
		} else if status == http.StatusUnauthorized {
			w.Header().Add(`WWW-Authenticate`, `Basic realm="Login"`)
		}
		http.Error(w, http.StatusText(status), status)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cfg.webAdminCookieName,
		Value:    base64.StdEncoding.EncodeToString([]byte(cookieVal)),
		HttpOnly: true,
		Secure:   false,
		Path:     `/`,
	})

	wa.path = strings.Trim(r.URL.Path, `/`)
	switch wa.path {
	case `audio`:
		wa.audio(w, r)
	case `audio_volume`:
		wa.audioVolume(w, r)
	case `lists`:
		wa.lists(w, r)
	case `lists_search`:
		wa.listsSearch(w, r)
	case `internet_radio`:
		wa.internetRadio(w, r)
	case `internet_radio_search`:
		wa.internetRadioSearch(w, r)
	case `internet_radio_update`:
		w.Header().Set(`Content-Type`, `text/plain`)
		internetRadio.updateChannel <- internetRadio.updateForced
		w.Write([]byte(`OK`))
	case `config`:
		wa.config(w, r)
	case `skin`:
		wa.skin(w, r)
	case `logs`:
		wa.logs(w, r)
	case `most_ordered_songs`:
		wa.mostOrderedSongs(w, r)
	case `chip_money_inserted`:
		wa.chipMoneyInserted(w, r)
	case `log_file_content`:
		wa.logFileContent(w, r)
	case `rotate_log`:
		w.Header().Set(`Content-Type`, `text/plain`)
		logger.rotate <- true
		w.Write([]byte(`OK`))
	default:
		wa.render(w)
	}
}

// Render pages
func (wa *WebAdmin) render(w http.ResponseWriter) {
	var err error

	w.Header().Set(`Content-Type`, `text/html`)

	t := template.New(`index` + wa.templateExtension)
	wa.functions[`RenderTemplate`] = func(name string, data interface{}) (template.HTML, error) {
		buf := bytes.NewBuffer([]byte{})
		err := t.ExecuteTemplate(buf, name, data)
		s := template.HTML(buf.String())
		return s, err
	}
	t = t.Funcs(wa.functions)

	files := []string{wa.templateDirectory + `index` + wa.templateExtension}
	if len(wa.path) > 1 {
		_, err := os.Stat(wa.templateDirectory + wa.path + wa.templateExtension)
		if os.IsNotExist(err) {
			logger.queue <- fmt.Sprint(err)
		} else {
			wa.templates = append(wa.templates, wa.path)
		}
	}
	// Parse wa.templates.
	if len(wa.templates) > 0 {
		for _, tpl := range wa.templates {
			files = append(files, wa.templateDirectory+tpl+wa.templateExtension)
		}
	}

	// t = template.Must(t.ParseFiles(files...))
	t, err = t.ParseFiles(files...)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	}

	// Translate messages from domain wa.path.
	var msgOK, msgErr []string
	if _, ok := locale.Domains[wa.path]; ok {
		for k, v := range wa.messageOK {
			tr := locale.GetD(wa.path, k, v...)
			if cfg.WebAdminLanguage != `en` && tr == k {
				tr = locale.GetD(`index`, k, v...)
			}
			msgOK = append(msgOK, tr)
		}
		for k, v := range wa.messageError {
			tr := locale.GetD(wa.path, k, v...)
			if cfg.WebAdminLanguage != `en` && tr == k {
				tr = locale.GetD(`index`, k, v...)
			}
			msgErr = append(msgErr, tr)
		}
	}

	err = t.Execute(w, struct {
		Templates    []string
		T            *Locale
		Stylesheets  []string
		MessageOK    []string
		MessageError []string
		JSV          string
		Data         interface{}
	}{
		wa.templates,
		locale,
		wa.stylesheets,
		msgOK,
		msgErr,
		fmt.Sprintf("%d", time.Now().Unix()),
		wa.data,
	})
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	}
}

// Audio configuration and controls.
func (wa *WebAdmin) audio(w http.ResponseWriter, r *http.Request) {
	var err error

	if r.Method == `POST` {
		err = r.ParseForm()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			wa.messageError[`Error saving data please try again`] = nil
		} else {
			wa.messageOK, wa.messageError = cfg.updateFromAudioWebAdmin(r)
		}
	}

	wa.data = struct {
		Cfg *Config
	}{
		&cfg,
	}

	wa.render(w)
}

// Audio volume.
func (wa *WebAdmin) audioVolume(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(`Content-Type`, `text/plain`)
	if r.Method == `POST` {
		if err := r.ParseForm(); err != nil {
			logger.queue <- fmt.Sprint(err)
		} else {
			if data, ok := jukebox.webAudioVolume(r); ok {
				w.Write([]byte(data))
				return
			}
		}
	}
	status := http.StatusBadRequest
	http.Error(w, http.StatusText(status), status)
}

// Lists configuration.
func (wa *WebAdmin) lists(w http.ResponseWriter, r *http.Request) {
	if r.Method == `POST` {
		err := r.ParseForm()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			wa.messageError[`Error saving data please try again`] = nil
		} else {
			// logger.queue <- fmt.Sprintf("%+v", r.Form)
			wa.messageOK, wa.messageError = lists.updateFromWebAdmin(r)
		}
	}

	wa.stylesheets = append(wa.stylesheets, `tree_view.css`)

	wa.data = struct {
		Lists                *Lists
		PlayListSongsPerSlot *[24]string
		LabelContentOptions  *[18]string
	}{
		lists,
		&lists.playListSongsPerSlot,
		&lists.labelContentOptions,
	}

	wa.render(w)
}

// Music lists.
func (wa *WebAdmin) listsSearch(w http.ResponseWriter, r *http.Request) {
	status := http.StatusBadRequest

	if r.Method == `GET` {
		if data, ok := lists.webAdminData(); ok {
			w.Header().Set(`Content-Type`, `application/json`)
			w.Write(data)
			return
		}
		status = http.StatusInternalServerError
	}
	w.Header().Set(`Content-Type`, `text/plain`)
	http.Error(w, http.StatusText(status), status)
}

// Internet radio configuration.
func (wa *WebAdmin) internetRadio(w http.ResponseWriter, r *http.Request) {
	var (
		ok        bool
		languages map[string]int
		tags      []string
		countries map[string]string
	)

	sk := ``
	sv := ``
	info, err := os.Stat(internetRadio.dbSQLName)
	if !os.IsNotExist(err) {
		sk = `Last download`
		sv = info.ModTime().Format(time.RFC1123)
	}

	if r.Method == `POST` {
		err := r.ParseForm()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			wa.messageError[`Error saving data please try again`] = nil
		} else {
			// logger.queue <- fmt.Sprintf("%+v", r.Form)
			wa.messageOK, wa.messageError = internetRadio.updateFromWebAdmin(r)
		}
	}

	wa.stylesheets = append(wa.stylesheets, `tree_view.css`)

	languages, tags, countries, ok = internetRadio.webAdminPage()
	if !ok {
		wa.messageError[`Internet radio database inaccessible`] = nil
	}

	wa.data = struct {
		Cfg          *Config
		Languages    map[string]int
		Tags         []string
		Countries    map[string]string
		LastDownload string
	}{
		&cfg,
		languages,
		tags,
		countries,
		locale.GetD(wa.path, sk, sv),
	}

	wa.render(w)
}

// Internet radio configuration.
func (wa *WebAdmin) internetRadioSearch(w http.ResponseWriter, r *http.Request) {
	status := http.StatusBadRequest

	if r.Method == `POST` {
		err := r.ParseForm()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
		} else {
			if data, ok := internetRadio.webAdminData(r); ok {
				w.Header().Set(`Content-Type`, `application/json`)
				w.Write(data)
				return
			}
			status = http.StatusInternalServerError
		}
	}
	w.Header().Set(`Content-Type`, `text/plain`)
	http.Error(w, http.StatusText(status), status)
}

// Application configuration.
func (wa *WebAdmin) config(w http.ResponseWriter, r *http.Request) {
	var (
		languages []string
	)

	// Load languages.
	err := filepath.Walk(
		`./templates`,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && len(info.Name()) == 2 {
				languages = append(languages, info.Name())
			}
			return nil
		})
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	}
	if len(languages) == 0 {
		languages = append(languages, `-`)
	}

	if r.Method == `POST` {
		err = r.ParseMultipartForm(32 << 20)
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			wa.messageError[`Error saving data please try again`] = nil
		} else {
			wa.messageOK, wa.messageError = cfg.updateFromWebAdmin(r, languages)
		}
	}

	musicSource := make(map[string]string)
	for k, v := range cfg.backgroundMusicSource {
		if !(k == `internet radio` && cfg.InternetRadioSelectedURL == ``) {
			musicSource[k] = v
		}
	}

	wa.data = struct {
		Cfg                   *Config
		Languages             []string
		BackgroundMusicSource *map[string]string
	}{
		&cfg,
		languages,
		&musicSource,
	}

	wa.render(w)
}

// Screen themes configuration.
func (wa *WebAdmin) skin(w http.ResponseWriter, r *http.Request) {
	const path = `./img`

	var (
		skins []string
	)

	// Load skins.
	files, err := ioutil.ReadDir(path)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	} else {
		for _, file := range files {
			if file.IsDir() {
				name := file.Name()
				if strings.HasPrefix(name, `skin_`) {
					skins = append(skins, name[5:])
				}
			}
		}
	}

	if r.Method == `POST` {
		err = r.ParseForm()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			wa.messageError[`Error saving data please try again`] = nil
		} else {
			wa.messageOK, wa.messageError = skin.updateFromWebAdmin(r, skins)
		}
	}

	wa.data = struct {
		Skin  string
		Skins []string
	}{
		cfg.Skin,
		skins,
	}

	wa.render(w)
}

// Log file setup and reports / statistics.
func (wa *WebAdmin) logs(w http.ResponseWriter, r *http.Request) {
	var err error

	if r.Method == `POST` {
		err = r.ParseForm()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			wa.messageError[`Error saving data please try again`] = nil
		} else {
			wa.messageOK, wa.messageError = logger.updateFromWebAdmin(r)
		}
	}

	wa.data = struct {
		Cfg *Config
	}{
		&cfg,
	}

	wa.render(w)
}

// Most ordered songs list.
func (wa *WebAdmin) mostOrderedSongs(w http.ResponseWriter, r *http.Request) {
	data := ``
	if r.Method == `POST` {
		err := r.ParseForm()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			data = `Error saving data please try again`
		} else {
			data = logger.webAdminOrderedSongsList(r)
		}
	}
	w.Header().Set(`Content-Type`, `text/html; charset=UTF-8`)
	w.Write([]byte(data))
}

// Amount of chip/money inserted list.
func (wa *WebAdmin) chipMoneyInserted(w http.ResponseWriter, r *http.Request) {
	data := ``
	if r.Method == `POST` {
		err := r.ParseForm()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			data = `Error saving data please try again`
		} else {
			data = logger.webAdminChipMoneyInsertedList(r)
		}
	}
	w.Header().Set(`Content-Type`, `text/html; charset=UTF-8`)
	w.Write([]byte(data))
}

// Content of the log file.
func (wa *WebAdmin) logFileContent(w http.ResponseWriter, r *http.Request) {
	var err error
	data := []byte(``)
	if r.Method == `GET` && cfg.LogFile != `` {
		data, err = ioutil.ReadFile(cfg.LogFile)
		if err != nil {
			data = []byte(``)
		}
	}
	w.Header().Set(`Content-Type`, `text/html; charset=UTF-8`)
	w.Write(data)
}

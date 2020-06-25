package main

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v2"
)

// Config configuration structure.
type Config struct {
	// Debug turn debugging on/off.
	Debug byte `yaml:"debug,omitempty"`
	// LogFile log file, stdout if empty.
	LogFile string `yaml:"log_file,omitempty"`
	// LogFileRotateSize log file rotation size in bytes.
	LogFileRotateSize int `yaml:"log_file_rotate_size,omitempty"`
	// VLCOptions options for initializing VLC library.
	VLCOptions []string `yaml:"vlc_options,omitempty"`
	// VolumeStep random and play lists volume increasing/decreasing step.
	VolumeStep int `yaml:"volume_step,omitempty"`
	// PlayListVolume inital value.
	PlayListVolume int `yaml:"play_list_volume,omitempty"`
	// PlayListVolumeUp keycode for increasing play list volume.
	PlayListVolumeUp []byte `yaml:"play_list_volume_up,omitempty"`
	// PlayListVolumeDown keycode for decreasing play list volume.
	PlayListVolumeDown []byte `yaml:"play_list_volume_down,omitempty"`
	// RandomListVolume inital value.
	RandomListVolume int `yaml:"random_list_volume,omitempty"`
	// RandomListVolumeUp keycode for increasing random list volume.
	RandomListVolumeUp []byte `yaml:"random_list_volume_up,omitempty"`
	// RandomListVolumeDown keycode for decreasing random list volume.
	RandomListVolumeDown []byte `yaml:"random_list_volume_down,omitempty"`
	// InternetRadioVolume inital value.
	InternetRadioVolume int `yaml:"internet_radio_volume,omitempty"`
	// InternetRadioVolumeUp keycode for increasing internet radio volume.
	InternetRadioVolumeUp []byte `yaml:"internet_radio_volume_up,omitempty"`
	// InternetRadioVolumeDown keycode for decreasing internet radio volume.
	InternetRadioVolumeDown []byte `yaml:"internet_radio_volume_down,omitempty"`
	// Internet radios database dump.
	InternetRadioDownloadURL string `yaml:"internet_radio_download_url,omitempty"`
	// InternetRadioSelectedURL selected internet radio station url.
	InternetRadioSelectedURL string `yaml:"internet_radio_selected_url,omitempty"`
	// InternetRadioSelectedName selected internet radio station name.
	InternetRadioSelectedName string `yaml:"internet_radio_selected_name,omitempty"`
	// Background music source.
	BackgroundMusic string `yaml:"background_music,omitempty"`
	// WebAdminUsername web administration username.
	WebAdminUsername string `yaml:"web_admin_username,omitempty"`
	// WebAdminPassword web administration hashed password.
	WebAdminPassword string `yaml:"web_admin_password,omitempty"`
	// WebAdminHTTPAddress web admininstration HTTP listen addresses.
	// If empty, there shall be no HTTP server.
	WebAdminHTTPAddress []string `yaml:"web_admin_http_address,omitempty"`
	// WebAdminHTTPSAddress web admininstration HTTPS listen addresses.
	// If empty, there shall be no HTTPS server.
	WebAdminHTTPSAddress []string `yaml:"web_admin_https_address,omitempty"`
	// WebAdminLanguage language of the web admin pages.
	WebAdminLanguage string `yaml:"web_admin_language,omitempty"`
	// TLSCertFile TLS cert file.
	TLSCertFile string `yaml:"tls_cert_file,omitempty"`
	// TLSKeyFile TLS key file.
	TLSKeyFile string `yaml:"tls_key_file,omitempty"`
	// Skin active screen theme.
	Skin string `yaml:"skin,omitempty"`
	// FreeSongsSelection free songs selectiion.
	FreeSongsSelection byte `yaml:"free_songs_selection,omitempty"`
	// Configuration file name.
	cfgFile string
	// Cookie name.
	webAdminCookieName string
	// Background music sources.
	backgroundMusicSource map[string]string
	// Selection source types.
	selectionSourceTypes map[string]string
}

var (
	cfg Config = Config{
		Debug:             1,
		LogFile:           `jukebox.log`,
		LogFileRotateSize: 1000000,
		VLCOptions: []string{
			`-A`,
			`alsa,none`,
			`--alsa-audio-device`,
			`default`,
			`--no-media-library`,
			// `--metadata-network-access`,
			`--no-metadata-network-access`,
			// `--no-ignore-config`,
			`--ignore-config`,
			`--no-video`,
			`--quiet`,
			`--play-and-exit`,
			`--audio-filter`,
			`normvol`,
			`--norm-buff-size=10`,
			`--norm-max-level=1.6`,
		},
		VolumeStep:               2,
		PlayListVolume:           100,
		PlayListVolumeUp:         []byte{27, 91, 67},
		PlayListVolumeDown:       []byte{27, 91, 68},
		RandomListVolume:         50,
		RandomListVolumeUp:       []byte{27, 91, 65},
		RandomListVolumeDown:     []byte{27, 91, 66},
		InternetRadioVolume:      50,
		InternetRadioVolumeUp:    []byte{27, 91, 53, 126},
		InternetRadioVolumeDown:  []byte{27, 91, 54, 126},
		InternetRadioDownloadURL: `http://www.radio-browser.info/backups/latest.sql.gz`,
		BackgroundMusic:          `list`,
		WebAdminUsername:         `admin`,
		WebAdminPassword:         `$2a$04$Ta6sfUK8l/WOmBli7hhyT.pgWI1Ac8aFs0vOIuhQza3OGjK57s7JS`,
		WebAdminHTTPAddress: []string{
			`:9090`,
		},
		WebAdminHTTPSAddress: []string{
			`:9191`,
		},
		WebAdminLanguage:   `en`,
		TLSCertFile:        `tls.crt`,
		TLSKeyFile:         `tls.key`,
		Skin:               `default`,
		FreeSongsSelection: 0,
		cfgFile:            `jukebox.yaml`,
		webAdminCookieName: `session_token`,
		backgroundMusicSource: map[string]string{
			`list`:           `BGMusic source list`,
			`internet radio`: `BGMusic source internet radio`,
		},
		selectionSourceTypes: map[string]string{
			`chip`:  `Song selection chip`,
			`money`: `Song selection money`,
			`free`:  `Song selection free`,
		},
	}
)

func hash(s string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.MinCost)
	if err != nil {
		return ``, err
	}
	return string(hash), nil
}

// Read configuration from the configuration file, if exist.
// If not, we have default values.
func (cf *Config) load() error {
	_, err := os.Stat(cf.cfgFile)
	if os.IsNotExist(err) {
		err = cf.save()
	} else {
		c, err := ioutil.ReadFile(cf.cfgFile)
		if err == nil {
			err = yaml.Unmarshal(c, &cf)
		}
	}
	return err
}

// Saves cfg in cfgFile.
func (cf Config) save() error {
	data, err := yaml.Marshal(cf)
	if err == nil {
		err = ioutil.WriteFile(cf.cfgFile, data, 0644)
	}
	return err
}

// Returns a new Config object, copy of the current Config object.
func (cf Config) copy() Config {
	var newCf = Config{
		Debug:                     cf.Debug,
		LogFile:                   cf.LogFile,
		LogFileRotateSize:         cf.LogFileRotateSize,
		VolumeStep:                cf.VolumeStep,
		PlayListVolume:            cf.PlayListVolume,
		RandomListVolume:          cf.RandomListVolume,
		InternetRadioVolume:       cf.InternetRadioVolume,
		InternetRadioDownloadURL:  cf.InternetRadioDownloadURL,
		InternetRadioSelectedURL:  cf.InternetRadioSelectedURL,
		InternetRadioSelectedName: cf.InternetRadioSelectedName,
		BackgroundMusic:           cf.BackgroundMusic,
		WebAdminUsername:          cf.WebAdminUsername,
		WebAdminPassword:          cf.WebAdminPassword,
		WebAdminLanguage:          cf.WebAdminLanguage,
		TLSCertFile:               cf.TLSCertFile,
		TLSKeyFile:                cf.TLSKeyFile,
		Skin:                      cf.Skin,
		FreeSongsSelection:        cf.FreeSongsSelection,
		cfgFile:                   cf.cfgFile,
		webAdminCookieName:        cf.webAdminCookieName,
	}

	newCf.VLCOptions = append(newCf.VLCOptions, cf.VLCOptions...)
	newCf.PlayListVolumeUp = append(newCf.PlayListVolumeUp, cf.PlayListVolumeUp...)
	newCf.PlayListVolumeDown = append(newCf.PlayListVolumeDown, cf.PlayListVolumeDown...)
	newCf.RandomListVolumeUp = append(newCf.RandomListVolumeUp, cf.RandomListVolumeUp...)
	newCf.RandomListVolumeDown = append(newCf.RandomListVolumeDown, cf.RandomListVolumeDown...)
	newCf.InternetRadioVolumeUp = append(newCf.InternetRadioVolumeUp, cf.InternetRadioVolumeUp...)
	newCf.InternetRadioVolumeDown = append(newCf.InternetRadioVolumeDown, cf.InternetRadioVolumeDown...)
	newCf.WebAdminHTTPAddress = append(newCf.WebAdminHTTPAddress, cf.WebAdminHTTPAddress...)
	newCf.WebAdminHTTPSAddress = append(newCf.WebAdminHTTPSAddress, cf.WebAdminHTTPSAddress...)
	newCf.backgroundMusicSource = make(map[string]string)
	for k, v := range cf.backgroundMusicSource {
		newCf.backgroundMusicSource[k] = v
	}
	newCf.selectionSourceTypes = make(map[string]string)
	for k, v := range cf.selectionSourceTypes {
		newCf.selectionSourceTypes[k] = v
	}

	return newCf
}

// Check password.
func (cf Config) checkPassword(p string) error {
	return bcrypt.CompareHashAndPassword([]byte(cf.WebAdminPassword), []byte(p))
}

// Check network address as expected by http server.
func (cf Config) checkNetworkAddress(a string) bool {
	sl := strings.Split(a, `:`)
	l := len(sl)
	if l == 1 {
		return false
	}
	if !(l == 2 && sl[0] == ``) {
		if ip := net.ParseIP(strings.Join(sl[:l-1], `:`)); ip == nil {
			return false
		}
	}
	port, err := strconv.Atoi(sl[l-1])
	if err != nil || port == 10000 || port < 0 || port > 65535 {
		return false
	}
	return true
}

// Updates configuration with values from web admin page.
func (cf *Config) updateFromWebAdmin(r *http.Request, languages []string) (msgOK, msgErr map[string][]interface{}) {
	var err error = nil
	msgOK = make(map[string][]interface{})
	msgErr = make(map[string][]interface{})
	newCf := cf.copy()
	changed := false
	password := r.FormValue(`new_password`)
	if password != `` {
		if password == r.FormValue(`confirm_new_password`) {
			err = cf.checkPassword(r.FormValue(`old_password`))
			if err == nil {
				password, err = hash(password)
				if err == nil {
					newCf.WebAdminPassword = password
					changed = true
				} else {
					logger.queue <- fmt.Sprint(err)
					msgErr[`Error saving data please try again`] = nil
				}
			} else {
				msgErr[`Invalid current password`] = nil
			}
		} else {
			msgErr[`New password and Confirm new password doesn't match`] = nil
		}
	}
	languageChange := false
	language := r.FormValue(`language`)
	if language != cf.WebAdminLanguage {
		for _, l := range languages {
			if l == language {
				newCf.WebAdminLanguage = language
				changed = true
				languageChange = true
			}
		}
	}
	reloadServers := false
	httpAddress := strings.Split(r.FormValue(`http_address`), "\r\n")
	newCf.WebAdminHTTPAddress = nil
	var e []string
	for _, v := range httpAddress {
		v = strings.TrimSpace(v)
		if len(v) == 0 {
			continue
		}
		if cf.checkNetworkAddress(v) {
			newCf.WebAdminHTTPAddress = append(newCf.WebAdminHTTPAddress, v)
		} else {
			e = append(e, template.HTMLEscapeString(v))
		}
	}
	if strings.Join(newCf.WebAdminHTTPAddress, ``) != strings.Join(cf.WebAdminHTTPAddress, ``) {
		changed = true
		reloadServers = true
	}
	if len(e) > 0 {
		msgErr[`Invalid HTTP address`] = []interface{}{e}
	}
	httpsAddress := strings.Split(r.FormValue(`https_address`), "\r\n")
	newCf.WebAdminHTTPSAddress = nil
	e = nil
	for _, v := range httpsAddress {
		v = strings.TrimSpace(v)
		if len(v) == 0 {
			continue
		}
		if cf.checkNetworkAddress(v) {
			newCf.WebAdminHTTPSAddress = append(newCf.WebAdminHTTPSAddress, v)
		} else {
			e = append(e, template.HTMLEscapeString(v))
		}
	}
	if strings.Join(newCf.WebAdminHTTPSAddress, ``) != strings.Join(cf.WebAdminHTTPSAddress, ``) {
		changed = true
		reloadServers = true
	}
	if len(e) > 0 {
		msgErr[`Invalid HTTPS address`] = []interface{}{e}
	}
	certFile, certHandler, err := r.FormFile(`tls_cert_file`)
	if err == nil {
		defer certFile.Close()
		cFile, err := os.OpenFile(`./`+certHandler.Filename, os.O_WRONLY|os.O_CREATE, 0644)
		if err == nil {
			defer cFile.Close()
			_, err = io.Copy(cFile, certFile)
			if err == nil {
				newCf.TLSCertFile = certHandler.Filename
				changed = true
				reloadServers = true
			}
		}
		if err != nil {
			msgErr[`Error saving TLS cert file`] = []interface{}{certHandler.Filename}
		}
	}
	if err != nil && err != http.ErrMissingFile {
		logger.queue <- fmt.Sprint(err)
	}
	keyFile, keyHandler, err := r.FormFile(`tls_key_file`)
	if err == nil {
		defer keyFile.Close()
		kFile, err := os.OpenFile(`./`+keyHandler.Filename, os.O_WRONLY|os.O_CREATE, 0600)
		if err == nil {
			defer kFile.Close()
			_, err = io.Copy(kFile, keyFile)
			if err == nil {
				newCf.TLSKeyFile = keyHandler.Filename
				changed = true
				reloadServers = true
			}
		}
		if err != nil {
			msgErr[`Error saving TLS key file`] = []interface{}{keyHandler.Filename}
		}
	}
	if err != nil && err != http.ErrMissingFile {
		logger.queue <- fmt.Sprint(err)
	}
	backgroundMusicChange := false
	backgroundMusic := strings.TrimSpace(r.FormValue(`background_music`))
	if backgroundMusic != cf.BackgroundMusic {
		if _, ok := cf.backgroundMusicSource[backgroundMusic]; ok {
			newCf.BackgroundMusic = backgroundMusic
			changed = true
			backgroundMusicChange = true
		}
	}
	var freeSongsSelection byte = 1
	freeSongsSelectionChange := false
	if r.FormValue(`free_songs_selection`) == `` {
		freeSongsSelection = 0
	}
	if freeSongsSelection != cfg.FreeSongsSelection {
		newCf.FreeSongsSelection = freeSongsSelection
		changed = true
		freeSongsSelectionChange = true
	}
	if changed {
		err = newCf.save()
		if err == nil {
			s := cf.BackgroundMusic
			*cf = newCf.copy()
			if backgroundMusicChange {
				jukebox.backgroundMusicChanged <- true
				logger.queue <- fmt.Sprintf("background music source changed from %s to %s", s, cf.BackgroundMusic)
			}
			if languageChange || freeSongsSelectionChange {
				if languageChange {
					l, err := NewLocale()
					if err == nil {
						locale = l
					} else {
						logger.queue <- fmt.Sprint(err)
					}
				}
				if userInterface.wsCanWrite {
					userInterface.screenMessageChannel <- `init`
				}
			}
			if reloadServers {
				webAdmin.action <- 'R'
			}
			msgOK[`Configuration changed`] = nil
		} else {
			logger.queue <- fmt.Sprint(err)
			msgErr[`Error saving data please try again`] = nil
		}
	}
	return
}

// Updates configuration with values from audio web admin page.
func (cf *Config) updateFromAudioWebAdmin(r *http.Request) (msgOK, msgErr map[string][]interface{}) {
	var err error = nil
	msgOK = make(map[string][]interface{})
	msgErr = make(map[string][]interface{})
	newCf := cf.copy()
	changed := false
	vlcOptions := strings.Split(r.FormValue(`vlc_options`), "\r\n")
	newCf.VLCOptions = nil
	for _, v := range vlcOptions {
		v = strings.TrimSpace(v)
		if len(v) == 0 {
			continue
		}
		newCf.VLCOptions = append(newCf.VLCOptions, v)
		changed = true
	}
	volumeStep, err := strconv.Atoi(strings.TrimSpace(r.FormValue(`volume_step`)))
	if err == nil {
		if volumeStep <= 0 {
			err = fmt.Errorf(`invalid volume step`)
		} else {
			if volumeStep != cf.VolumeStep {
				newCf.VolumeStep = volumeStep
				changed = true
			}
		}
	}
	if err != nil {
		msgErr[`Invalid volume step`] = []interface{}{template.HTMLEscapeString(r.FormValue(`volume_step`))}
	}
	playListVolume, err := strconv.Atoi(strings.TrimSpace(r.FormValue(`play_list_volume`)))
	if err == nil {
		if playListVolume < 0 {
			err = fmt.Errorf(`invalid play list volume`)
		} else {
			if playListVolume != cf.PlayListVolume {
				newCf.PlayListVolume = playListVolume
				changed = true
			}
		}
	}
	if err != nil {
		msgErr[`Invalid play list volume`] = []interface{}{template.HTMLEscapeString(r.FormValue(`play_list_volume`))}
	}
	randomListVolume, err := strconv.Atoi(strings.TrimSpace(r.FormValue(`random_list_volume`)))
	if err == nil {
		if randomListVolume < 0 {
			err = fmt.Errorf(`invalid random list volume`)
		} else {
			if randomListVolume != cf.RandomListVolume {
				newCf.RandomListVolume = randomListVolume
				changed = true
			}
		}
	}
	if err != nil {
		msgErr[`Invalid random list volume`] = []interface{}{template.HTMLEscapeString(r.FormValue(`random_list_volume`))}
	}
	internetRadioVolume, err := strconv.Atoi(strings.TrimSpace(r.FormValue(`internet_radio_volume`)))
	if err == nil {
		if internetRadioVolume < 0 {
			err = fmt.Errorf(`invalid internet radio volume`)
		} else {
			if internetRadioVolume != cf.InternetRadioVolume {
				newCf.InternetRadioVolume = internetRadioVolume
				changed = true
			}
		}
	}
	if err != nil {
		msgErr[`Invalid radio volume`] = []interface{}{template.HTMLEscapeString(r.FormValue(`radio_volume`))}
	}
	if changed {
		err = newCf.save()
		if err == nil {
			*cf = newCf.copy()
			msgOK[`Configuration changed`] = nil
		} else {
			logger.queue <- fmt.Sprint(err)
			msgErr[`Error saving data please try again`] = nil
		}
	}
	return
}

package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// UserInterface User interface structure.
type UserInterface struct {
	ScreenMessageType    string `json:"messageType"`
	ScreenMessageData    string `json:"messageData,omitempty"`
	screenMessageChannel chan string
	ws                   *websocket.Conn
	wsReadTimeout        time.Duration
	wsWriteTimeout       time.Duration
	wsPingPeriod         time.Duration
	wsCanWrite           bool
}

var userInterface *UserInterface

// NewUserInterface creates new user interface.
func NewUserInterface() *UserInterface {
	ui := &UserInterface{
		screenMessageChannel: make(chan string),
		wsReadTimeout:        10 * time.Second,
		wsWriteTimeout:       5 * time.Second,
		wsPingPeriod:         3 * time.Second,
	}

	// Address to listen on.
	a := `127.0.0.1:10000`

	// Router.
	mux := http.NewServeMux()
	mux.Handle(`/css/`, http.StripPrefix(`/css/`, http.FileServer(http.Dir(`css`))))
	mux.Handle(`/js/`, http.StripPrefix(`/js/`, http.FileServer(http.Dir(`js`))))
	mux.Handle(`/img/`, http.StripPrefix(`/img/`, http.FileServer(http.Dir(`img`))))
	mux.Handle(`/art/`, http.StripPrefix(`/art/`, http.FileServer(http.Dir(`art`))))
	mux.HandleFunc(`/app`, func(w http.ResponseWriter, r *http.Request) {
		r.Header.Add(`Cache-Control`, `private, no-cache, no-store, must-revalidate`)
		logger.queue <- fmt.Sprintf(`jukebox screen request to serve %s`, `./templates/app_`+cfg.Skin+`.html`)
		http.ServeFile(w, r, `./templates/app_`+cfg.Skin+`.html`)
	})
	mux.HandleFunc(`/data`, ui.screen)

	// Jukebox screen HTTP server goroutine.
	go func() {
		logger.queue <- fmt.Sprintf("Starting jukebox screen HTTP server on %s ...", a)
		server := &http.Server{
			Addr:         a,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			IdleTimeout:  5 * time.Second,
		}
		go func() {
			for {
				if _, err := net.DialTimeout(`tcp`, a, time.Duration(1*time.Second)); err == nil {
					if err := ioutil.WriteFile("./jukebox.env", []byte(`export JUKEBOX_KIOSK=`+a), 0644); err != nil {
						logger.queue <- fmt.Sprint(err)
					} else {
						logger.queue <- fmt.Sprintf("jukebox screen HTTP server on %s started", a)
						return
					}
				}
				logger.queue <- `waiting jukebox screen to start...`
				time.Sleep(1 * time.Second)
			}
		}()
		logger.queue <- fmt.Sprint(server.ListenAndServe())
	}()

	return ui
}

// Keyboard input goroutine.
func (ui *UserInterface) keyboard() {
	const fifoFile = `jukebox.fifo`

	var (
		err     error
		reader  *bufio.Reader
		r, w    *os.File
		keyCode []byte
	)
	if fifoFile == `` {
		reader = bufio.NewReader(os.Stdin)
	} else {
		err = func() error {
			os.Remove(fifoFile)
			err := syscall.Mkfifo(fifoFile, 0666)
			if err != nil {
				return err
			}
			r, err = os.OpenFile(fifoFile, os.O_CREATE, os.ModeNamedPipe)
			if err != nil {
				return err
			}
			w, err = os.OpenFile(fifoFile, os.O_WRONLY, os.ModeNamedPipe)
			if err != nil {
				return err
			}
			return nil
		}()
		defer func() {
			r.Close()
			w.Close()
		}()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return
		}
		reader = bufio.NewReader(r)
	}
	for {
		if keyCode, err = reader.ReadBytes('\n'); err != nil {
			logger.queue <- fmt.Sprint(err)
			continue
		}
		keyCode = keyCode[:len(keyCode)-1]
		if len(keyCode) == 0 {
			continue
		}
		if cfg.Debug != 0 {
			logger.queue <- fmt.Sprintf("received from keyboard %v", keyCode)
		}
		if keyCode[0] == '~' {
			ui.chipOrMoneyInserted(1, 1)
			continue
		}
		if bytes.HasSuffix(keyCode, cfg.RandomListVolumeUp) {
			jukebox.randomListVolumeChannel <- bytes.Count(keyCode, cfg.RandomListVolumeUp) * cfg.VolumeStep
		} else if bytes.HasSuffix(keyCode, cfg.RandomListVolumeDown) {
			jukebox.randomListVolumeChannel <- -(bytes.Count(keyCode, cfg.RandomListVolumeDown) * cfg.VolumeStep)
		} else if bytes.HasSuffix(keyCode, cfg.PlayListVolumeUp) {
			jukebox.playListVolumeChannel <- bytes.Count(keyCode, cfg.PlayListVolumeUp) * cfg.VolumeStep
		} else if bytes.HasSuffix(keyCode, cfg.PlayListVolumeDown) {
			jukebox.playListVolumeChannel <- -(bytes.Count(keyCode, cfg.PlayListVolumeDown) * cfg.VolumeStep)
		} else if bytes.HasSuffix(keyCode, cfg.InternetRadioVolumeUp) {
			jukebox.internetRadioVolumeChannel <- bytes.Count(keyCode, cfg.InternetRadioVolumeUp) * cfg.VolumeStep
		} else if bytes.HasSuffix(keyCode, cfg.InternetRadioVolumeDown) {
			jukebox.internetRadioVolumeChannel <- -(bytes.Count(keyCode, cfg.InternetRadioVolumeDown) * cfg.VolumeStep)
		} else {
			s, l, err := lists.chosenSongOrList(keyCode)
			if err == nil {
				if l {
					logger.queue <- fmt.Sprintf("selected new play list %v", keyCode)
				} else {
					if cfg.Debug != 0 {
						logger.queue <- fmt.Sprintf("selected from play list \"%s\"", s)
						logger.orderedSongsQueue <- s
					}
					jukebox.playListChannel <- s
				}
			} else {
				logger.queue <- fmt.Sprint(err)
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Chip/money inserted action.
func (ui *UserInterface) chipOrMoneyInserted(amount, numberOfSongs int) {
	ui.screenMessageChannel <- fmt.Sprintf("coin:%d", numberOfSongs)
	logger.chipMoneyQueue <- fmt.Sprintf("%s,%d", gpio.ChipMoneyType, amount)
	logger.queue <- fmt.Sprintf("%s selection made, number of songs %d", gpio.ChipMoneyType, numberOfSongs)
}

// Chip/money retrieve (cancel) action.
func (ui *UserInterface) coinRetrieve() {
	logger.queue <- gpio.ChipMoneyType + ` retrieve request`
}

// Process HTTP screen jukebox screen websocket.
func (ui *UserInterface) screen(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "http://"+r.Host {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	var (
		err error
		wg  sync.WaitGroup
	)

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	ui.ws, err = upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer func() {
		// Cleanup websocket connection.
		if err := ui.ws.Close(); err != nil {
			logger.queue <- fmt.Sprint(err)
		}
	}()

	ui.ws.SetPongHandler(func(string) error {
		ui.ws.SetReadDeadline(time.Now().Add(ui.wsReadTimeout))
		return nil
	})

	// Websocket writer.
	wg.Add(1)
	go func() {
		var err error

		pingTicker := time.NewTicker(ui.wsPingPeriod)

		ui.wsCanWrite = true
		defer func() {
			ui.wsCanWrite = false
			pingTicker.Stop()
			wg.Done()
		}()

		for {
			select {
			case <-pingTicker.C:
				ui.ScreenMessageType = `ping`
				err = ui.ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(ui.wsWriteTimeout))
			case ui.ScreenMessageType = <-ui.screenMessageChannel:
				if ui.ScreenMessageType == `init` {
					err = ui.screenInit()
				} else if ui.ScreenMessageType == `browseInit` {
					err = ui.screenBrowseInit()
				} else if strings.HasPrefix(ui.ScreenMessageType, `coin`) {
					ui.ScreenMessageData = ui.ScreenMessageType[5:]
					ui.ScreenMessageType = `coin`
				} else if ui.ScreenMessageType == `modalImage` {
					d := ``
					a := strings.Split(ui.ScreenMessageData, `#`)
					if len(a) == 2 && len(a[1]) > 2 {
						if slot, ok := lists.PlayList[a[1][:1]]; ok {
							if song, ok := slot[a[1][2:]]; ok {
								d = ui.rawImage(a[1], song.Icon)
							}
						}
					}
					ui.ScreenMessageData = a[0] + `#` + d
				} else {
					ui.ScreenMessageData = ``
				}
			}

			if err == nil && ui.ScreenMessageType != `ping` {
				err = ui.ws.WriteJSON(ui)
			}
			if err != nil {
				logger.queue <- fmt.Sprintf("jukebox screen error writing '%s': %+v", ui.ScreenMessageType, err)
				return
			}
		}
	}()

	// Initialize jukebox screen.
	ui.screenMessageChannel <- `init`

	// Websocket reader.
	wg.Add(1)
	for {
		ui.ws.SetReadDeadline(time.Now().Add(ui.wsReadTimeout))
		err = ui.ws.ReadJSON(ui)
		if err != nil {
			logger.queue <- fmt.Sprintf("jukebox screen error reading json: %+v", err)
			wg.Done()
			break
		}
		logger.queue <- fmt.Sprintf("jukebox screen request: '%s'", ui.ScreenMessageType)
		if ui.ScreenMessageType == `play` {
			// Play selected songs.
			for _, song := range strings.Split(ui.ScreenMessageData, `,`) {
				if len(song) != 3 || song[1] != '-' {
					logger.queue <- fmt.Sprintf("jukebox screen invalid song format '%s'", song)
					continue
				}
				s, l, err := lists.chosenSongOrList([]byte{song[2]}, string(song[0]))
				if err == nil {
					if l {
						logger.queue <- fmt.Sprintf("jukebox screen invalid song '%s'", song)
						continue
					}
					if cfg.Debug != 0 {
						logger.queue <- fmt.Sprintf("jukebox screen selected from play list \"%s\"", s)
						logger.orderedSongsQueue <- s
					}
					jukebox.playListChannel <- s
				} else {
					logger.queue <- fmt.Sprintf("jukebox screen selected song error %v", err)
				}
			}
		} else if ui.ScreenMessageType == `browseInit` {
			ui.screenMessageChannel <- ui.ScreenMessageType
		} else if ui.ScreenMessageType == `browse_play` {
			// Play selected songs from broswer screen.
			for _, song := range strings.Split(ui.ScreenMessageData, `,`) {
				fileName := lists.localDir + song
				if song != `` && lists.checkSong(fileName) {
					if cfg.Debug != 0 {
						logger.queue <- fmt.Sprintf("jukebox screen selected from screen browser \"%s\"", fileName)
						logger.orderedSongsQueue <- fileName
					}
					jukebox.playListChannel <- fileName
				} else {
					logger.queue <- fmt.Sprintf("jukebox screen browser invalid song '%s'", song)
				}
			}
		} else if ui.ScreenMessageType == `coin` {
			// Retrieve chip/money
			ui.coinRetrieve()
		} else if ui.ScreenMessageType == `modalImage` {
			ui.screenMessageChannel <- ui.ScreenMessageType
		}
	}

	wg.Wait()
}

func (ui *UserInterface) screenInit() error {
	type songs struct {
		Songs               map[string]map[string]map[string]string
		PlayLists           []string
		SelectionSource     string
		LabelContent        string
		ErrorText           string
		ListText            string
		UsageText           string
		UsageSongText       string
		UsageIconText       string
		SongSelectedText    string
		SongsText           string
		CancelSelectionText string
		PlaySelectionText   string
		ChipText            string
		MoneyText           string
	}

	pl := songs{
		LabelContent:        lists.LabelContent,
		ErrorText:           locale.GetD(`ui`, `Error`),
		ListText:            locale.GetD(`ui`, `Change list`),
		UsageText:           locale.GetD(`ui`, `Usage`),
		UsageSongText:       locale.GetD(`ui`, `Usage song`),
		UsageIconText:       locale.GetD(`ui`, `Usage icon`),
		SongSelectedText:    locale.GetD(`ui`, `Song selected`),
		SongsText:           locale.GetD(`ui`, `Songs`),
		CancelSelectionText: locale.GetD(`ui`, `Cancel`),
		PlaySelectionText:   locale.GetD(`ui`, `Play`),
		ChipText:            locale.GetD(`ui`, `Chip`),
		MoneyText:           locale.GetD(`ui`, `Money`),
	}
	// Selection source
	if cfg.FreeSongsSelection == 1 {
		pl.SelectionSource = `free`
	} else {
		pl.SelectionSource = gpio.ChipMoneyType
	}
	pl.Songs = make(map[string]map[string]map[string]string)
	for k, v := range lists.ShowPlayList {
		if v {
			pl.PlayLists = append(pl.PlayLists, k)
			pl.Songs[k] = make(map[string]map[string]string)
			for l, s := range lists.PlayList[k] {
				pl.Songs[k][l] = make(map[string]string)
				pl.Songs[k][l][`Name`] = s.Name
				pl.Songs[k][l][`Author`] = s.Author
				if s.Icon == `` {
					pl.Songs[k][l][`Icon`] = ``
				} else {
					pl.Songs[k][l][`Icon`] = ui.rawImage(k+`_`+l+`_thumb`, s.Icon)
				}
			}
		}
	}
	sort.Strings(pl.PlayLists)
	plm, err := json.Marshal(pl)
	if err == nil {
		ui.ScreenMessageData = string(plm)
	}

	return err
}

func (ui *UserInterface) screenBrowseInit() error {
	type item struct {
		ID     string  `json:"id"`
		Name   string  `json:"name"`
		File   string  `json:"file"`
		Folder string  `json:"folder"`
		Parent *string `json:"parent"`
	}

	var (
		err  error
		data []item
	)

	i := 1
	dirs := make(map[string]int)
	err = filepath.Walk(lists.realLocalDir, func(path string, info os.FileInfo, err error) error {
		defer func() {
			i++
		}()

		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return filepath.SkipDir
		}
		if path == lists.realLocalDir {
			return nil
		}

		l := len(lists.BrowseList)
		ok := true
		path = strings.TrimPrefix(path, lists.realLocalDir)
		if info.IsDir() && l > 0 {
			ok = false
			for _, bl := range lists.BrowseList {
				if strings.HasPrefix(bl+`/`, path+`/`) || strings.HasPrefix(path+`/`, bl+`/`) {
					ok = true
					break
				}
			}
		}
		if !ok {
			return filepath.SkipDir
		}

		folder := `0`
		if info.IsDir() {
			folder = `1`
			if _, ok := dirs[path]; !ok {
				dirs[path] = i
			}
		}
		if len(path) > 0 {
			dir := filepath.Dir(path)
			d := item{
				strconv.Itoa(i),
				filepath.Base(path),
				dir,
				folder,
				nil,
			}
			if v, ok := dirs[dir]; ok {
				sv := strconv.Itoa(v)
				d.Parent = &sv
			}
			data = append(data, d)
		}

		return nil
	})
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	}

	if len(data) > 0 {
		bl, err := json.Marshal(&data)
		if err == nil {
			ui.ScreenMessageData = string(bl)
		} else {
			ui.ScreenMessageData = `[]`
		}
	} else {
		ui.ScreenMessageData = `[]`
	}

	return err
}

func (ui *UserInterface) rawImage(baseName, songIcon string) (imgSrc string) {
	file, err := os.Open(lists.artworkDir + baseName + `.` + songIcon)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	} else {
		if songIcon == `jpg` || songIcon == `jpe` {
			imgSrc = `jpeg`
		} else {
			imgSrc = songIcon
		}
		imgSrc += `;base64,` + base64.StdEncoding.EncodeToString(content)
	}

	return
}

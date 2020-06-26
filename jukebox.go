package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	vlc "github.com/adrg/libvlc-go/v3"
)

// Jukebox the jukebox structure.
type Jukebox struct {
	player                     *exec.Cmd
	playerStdin                io.WriteCloser
	playerResponse             chan string
	vlcPlayer                  *vlc.Player
	randomListVolume           int
	randomListVolumeChannel    chan int
	randomListChannel          chan string
	playListVolume             int
	playListVolumeChannel      chan int
	playListChannel            chan string
	internetRadioVolume        int
	internetRadioVolumeChannel chan int
	internetRadioChanged       chan bool
	backgroundMusicChanged     chan bool
	songToPlay                 chan string
	songFinished               chan bool
	choiceMade                 chan bool
	randomListChanged          chan bool
	currentAudioVolumeChannel  chan int
}

var jukebox *Jukebox

// NewJukebox creates new jukebox.
func NewJukebox() (*Jukebox, error) {
	var (
		err error
		j   *Jukebox
	)

	j = &Jukebox{
		playerResponse: make(chan string),
	}
	j.vlcPlayer, err = vlc.NewPlayer()
	if err != nil {
		return nil, err
	}
	err = j.createPlayer()
	if err != nil {
		return nil, err
	}
	j.randomListVolume = cfg.RandomListVolume
	j.randomListVolumeChannel = make(chan int, 1)
	j.randomListChannel = make(chan string, 1)
	j.playListVolume = cfg.PlayListVolume
	j.playListVolumeChannel = make(chan int, 1)
	j.playListChannel = make(chan string, 2048)
	j.internetRadioVolume = cfg.InternetRadioVolume
	j.internetRadioVolumeChannel = make(chan int, 1)
	j.internetRadioChanged = make(chan bool, 1)
	j.backgroundMusicChanged = make(chan bool, 1)
	j.songToPlay = make(chan string, 1)
	j.songFinished = make(chan bool)
	j.choiceMade = make(chan bool)
	j.randomListChanged = make(chan bool)
	j.currentAudioVolumeChannel = make(chan int)

	return j, nil
}

// Create the player
func (j *Jukebox) createPlayer() (err error) {
	if j.player != nil {
		return
	}

	defer func() {
		if err == nil {
			return
		}
		if j.player != nil {
			j.player.Process.Kill()
		}
	}()

	j.player = exec.Command(`rvlc`, cfg.VLCOptions...)
	j.player.Env = os.Environ()
	j.player.SysProcAttr = &syscall.SysProcAttr{
		// Pdeathsig: syscall.SIGKILL,
		Pdeathsig: syscall.SIGTERM,
	}

	if j.playerStdin, err = j.player.StdinPipe(); err != nil {
		return
	}
	defer func() {
		if e := j.playerStdin.Close(); e != nil {
			logger.queue <- fmt.Sprint(e)
		}
	}()

	stdout, err := j.player.StdoutPipe()
	if err != nil {
		return
	}
	defer func() {
		if e := stdout.Close(); e != nil {
			logger.queue <- fmt.Sprint(e)
		}
	}()

	if err = j.player.Start(); err != nil {
		return
	}

	go func() {
		r := bufio.NewReader(stdout)
		for {
			s, _ := r.ReadString('\n')
			j.playerResponse <- s[:len(s)-1]
		}
	}()

	go func() {
		if e := j.player.Wait(); e != nil {
			logger.queue <- fmt.Sprint(e)
		}
		j.player = nil
	}()

	return
}

// Sets the volume of the player.
func (j *Jukebox) setVolume(volume int) {
	if volume < 0 {
		volume = 0
	}
	if err := j.vlcPlayer.SetVolume(volume); err != nil {
		logger.queue <- fmt.Sprint(err)
		if err := j.vlcPlayer.SetVolume(100); err != nil {
			logger.queue <- fmt.Sprint(err)
		}
	}
}

// Returns true if internet radio is selected for background music.
func (j *Jukebox) internetRadioSelected() bool {
	return cfg.BackgroundMusic == `internet radio` && cfg.InternetRadioSelectedURL != ``
}

// Release media player, stop playing.
func (j *Jukebox) releaseMedia() {
	m, err := j.vlcPlayer.Media()
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	} else {
		m.Release()
	}
}

// The player.
func (j *Jukebox) play() {
	var (
		err     error
		media   *vlc.Media
		manager *vlc.EventManager
		eventID vlc.EventID
	)

	// Create player.
	cmd := exec.Command(`rvlc`, cfg.VLCOptions...)
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Pdeathsig: syscall.SIGKILL,
		Pdeathsig: syscall.SIGTERM,
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	if err = cmd.Start(); err != nil {
		return
	}

	output := make(chan string)
	defer close(output)
	go func() {
		r := bufio.NewReader(stdout)
		for {
			s, _ := r.ReadString('\n')
			output <- s[:len(s)-1]
		}
	}()

	go func() {
		if err = cmd.Wait(); err != nil {
			logger.queue <- fmt.Sprint(err)
		}
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			io.WriteString(stdin, "is_playing\n")
		case o := <-output:
			if o == `0` {
				s := "clear\nloop off\n"
				if cfg.BackgroundMusic == `list` {
					s += "repeat on\nrandom on\nadd "
					s += lists.randomPlayListFile
				} else if cfg.BackgroundMusic == `internet radio` {
					if jukebox.internetRadioSelected() {
						s += "repeat off\nrandom off\nadd "
						s += cfg.InternetRadioSelectedURL
					} else {
						logger.queue <- `no station selected for playing internet radio`
						s += "repeat on\nrandom on\nadd "
						s += lists.randomPlayListFile
					}
				}
				io.WriteString(stdin, fmt.Sprintf("%s\n", s))
			}
		}
	}

	/*
	*
	* OLD !!!
	 */

	if manager, err = j.vlcPlayer.EventManager(); err != nil {
		logger.queue <- fmt.Sprint(err)
		return
	}
	callback := func(event vlc.Event, userData interface{}) {
		j.songFinished <- true
	}
	eventID, err = manager.Attach(vlc.MediaPlayerEndReached, callback, nil)
	if err != nil {
		return
	}
	defer manager.Detach(eventID)
	for {
		select {
		case s := <-j.songToPlay:
			if strings.HasPrefix(s, `http://`) || strings.HasPrefix(s, `https://`) {
				media, err = j.vlcPlayer.LoadMediaFromURL(s)
			} else {
				media, err = j.vlcPlayer.LoadMediaFromPath(s)
			}
			if err != nil {
				logger.queue <- fmt.Sprint(err)
				continue
			}
			defer media.Release()
			if err = j.vlcPlayer.Play(); err != nil {
				logger.queue <- fmt.Sprint(err)
				continue
			}
		}
	}
}

func (j *Jukebox) webAudioVolume(r *http.Request) (string, bool) {
	var err error

	txt := ``
	action := r.FormValue(`audio_volume_value`)
	switch action {
	case `save`:
		playListVolume := cfg.PlayListVolume
		randomListVolume := cfg.RandomListVolume
		internetRadioVolume := cfg.InternetRadioVolume
		cfg.PlayListVolume = j.playListVolume
		cfg.RandomListVolume = j.randomListVolume
		cfg.InternetRadioVolume = j.internetRadioVolume
		err = cfg.save()
		if err == nil {
			txt = locale.GetD(`index`, `Configuration changed`)
		} else {
			cfg.PlayListVolume = playListVolume
			cfg.RandomListVolume = randomListVolume
			cfg.InternetRadioVolume = internetRadioVolume
		}
	case `audio_volume_plus`:
		j.currentAudioVolumeChannel <- cfg.VolumeStep
	case `audio_volume_minus`:
		j.currentAudioVolumeChannel <- -cfg.VolumeStep
	default:
		err = fmt.Errorf("invalid request: audio_volume_value=\"%s\"", action)
	}

	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return ``, false
	}

	if txt == `` {
		return ``, true
	}
	return `msg=` + txt +
		`;play_list_volume=` + strconv.Itoa(cfg.PlayListVolume) +
		`;random_list_volume=` + strconv.Itoa(cfg.RandomListVolume) +
		`;internet_radio_volume=` + strconv.Itoa(cfg.InternetRadioVolume), true
}

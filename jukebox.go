package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	vlc "github.com/adrg/libvlc-go/v3"
)

// Jukebox the jukebox structure.
type Jukebox struct {
	player                     *vlc.Player
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

	j = &Jukebox{}
	j.player, err = vlc.NewPlayer()
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

// Sets the volume of the player.
func (j *Jukebox) setVolume(volume int) {
	if volume < 0 {
		volume = 0
	}
	if err := j.player.SetVolume(volume); err != nil {
		logger.queue <- fmt.Sprint(err)
		if err := j.player.SetVolume(100); err != nil {
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
	m, err := j.player.Media()
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

	if manager, err = j.player.EventManager(); err != nil {
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
				media, err = j.player.LoadMediaFromURL(s)
			} else {
				media, err = j.player.LoadMediaFromPath(s)
			}
			if err != nil {
				logger.queue <- fmt.Sprint(err)
				continue
			}
			defer media.Release()
			if err = j.player.Play(); err != nil {
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

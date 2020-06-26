package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// Jukebox the jukebox structure.
type Jukebox struct {
	singleSongToPlay           chan string
	randomListVolume           int
	randomListVolumeChannel    chan int
	playListVolume             int
	playListVolumeChannel      chan int
	playListChannel            chan string
	internetRadioVolume        int
	internetRadioVolumeChannel chan int
	internetRadioChanged       chan bool
	backgroundMusicChanged     chan bool
	randomListChanged          chan bool
	currentAudioVolumeChannel  chan int
}

var (
	jukebox = Jukebox{
		singleSongToPlay:           make(chan string, 1),
		randomListVolume:           cfg.RandomListVolume,
		randomListVolumeChannel:    make(chan int, 1),
		playListVolume:             cfg.PlayListVolume,
		playListVolumeChannel:      make(chan int, 1),
		playListChannel:            make(chan string, 2048),
		internetRadioVolume:        cfg.InternetRadioVolume,
		internetRadioVolumeChannel: make(chan int, 1),
		internetRadioChanged:       make(chan bool, 1),
		backgroundMusicChanged:     make(chan bool, 1),
		randomListChanged:          make(chan bool),
		currentAudioVolumeChannel:  make(chan int),
	}
)

// Sets the volume of the player.
func (j *Jukebox) setVolume(volume int, gain int) string {
	if volume < 0 {
		volume = 0
	}
	return string(int((float64(volume+gain) / 100.0) * 256.0))
}

// The player.
func (j *Jukebox) play() {
	var (
		err error
	)

	defer func() {
		if err != nil {
			logger.queue <- fmt.Sprint(err)
		}
	}()

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

	go func(err error) {
		if err = cmd.Wait(); err != nil {
			logger.queue <- fmt.Sprint(err)
		}
	}(err)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	ctrl := ``
	singleSong := ``
	isPlaying := `0`
	backgroundPlaying := false
	internetRadioPlaying := false
	for {
		select {
		case singleSong = <-j.singleSongToPlay:
			ctrl = "clear\nloop off\nrepeat off\nrandom off\nvolume " + j.setVolume(cfg.PlayListVolume, 0) + "\nadd " + singleSong
		case song := <-j.playListChannel:
			ctrl = ``
			if backgroundPlaying {
				backgroundPlaying = false
				ctrl = "clear\nloop off\nrepeat off\nrandom off\n"
			}
			ctrl += "volume " + j.setVolume(cfg.PlayListVolume, 0) + "\nadd " + song
		case <-j.randomListChanged:
			lists.randomList()
			if backgroundPlaying && !internetRadioPlaying {
				ctrl = "clear\n"
			}
		case <-j.internetRadioChanged:
			if backgroundPlaying && internetRadioPlaying {
				ctrl = "clear\n"
			}
		case <-j.backgroundMusicChanged:
			if backgroundPlaying {
				ctrl = "clear\n"
			}
		case gain := <-j.playListVolumeChannel:
			j.playListVolume += gain
			if isPlaying == `1` && !backgroundPlaying {
				ctrl = `volume ` + j.setVolume(j.playListVolume, gain) + "\n"
			}
			if cfg.Debug != 0 {
				logger.queue <- fmt.Sprintf("playListVolume changed by %d, current value %d", gain, j.playListVolume)
			}
		case gain := <-j.randomListVolumeChannel:
			j.randomListVolume += gain
			if isPlaying == `1` && backgroundPlaying && !internetRadioPlaying {
				ctrl = `volume ` + j.setVolume(j.randomListVolume, gain) + "\n"
			}
			if cfg.Debug != 0 {
				logger.queue <- fmt.Sprintf("randomListVolume changed by %d, current value %d", gain, j.randomListVolume)
			}
		case gain := <-j.internetRadioVolumeChannel:
			j.internetRadioVolume += gain
			if isPlaying == `1` && backgroundPlaying && internetRadioPlaying {
				ctrl = `volume ` + j.setVolume(j.internetRadioVolume, gain) + "\n"
			}
			if cfg.Debug != 0 {
				logger.queue <- fmt.Sprintf("internetRadioVolume changed by %d, current value %d", gain, j.internetRadioVolume)
			}
		case gain := <-jukebox.currentAudioVolumeChannel:
			if backgroundPlaying {
				j.playListVolume += gain
				if isPlaying == `1` {
					ctrl = `volume ` + j.setVolume(j.playListVolume, gain) + "\n"
				}
			} else {
				if internetRadioPlaying {
					j.internetRadioVolume += gain
					if isPlaying == `1` {
						ctrl = `volume ` + j.setVolume(j.internetRadioVolume, gain) + "\n"
					}
				} else {
					j.randomListVolume += gain
					if isPlaying == `1` {
						ctrl = `volume ` + j.setVolume(j.randomListVolume, gain) + "\n"
					}
				}
			}
		case <-ticker.C:
			ctrl = "is_playing\n"
		case isPlaying = <-output:
			if isPlaying == `0` {
				if singleSong != `` {
					cmd.Process.Kill()
					return
				}
				backgroundPlaying = true
				s := ``
				if cfg.BackgroundMusic == `internet radio` {
					if cfg.InternetRadioSelectedURL != `` {
						internetRadioPlaying = true
						s = "repeat off\nrandom off\nadd " + cfg.InternetRadioSelectedURL
						logger.queue <- fmt.Sprintf("playing internet radio station %s", cfg.InternetRadioSelectedName)
					} else {
						logger.queue <- `no station selected for playing internet radio`
					}
				}
				if s == `` {
					internetRadioPlaying = false
					s = "repeat on\nrandom on\nadd " + lists.randomPlayListFile
					logger.queue <- `playing from random list`
				}
				ctrl = "clear\nloop off\n" + s
			}
		}

		if ctrl != `` {
			io.WriteString(stdin, ctrl+"\n")
			ctrl = ``
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

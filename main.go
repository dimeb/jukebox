package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	flagFile          string
	flagHashString    string
	flagDefaultConfig bool
	flagUniOfficeTest bool
)

func init() {
	flag.Usage = func() {
		fmt.Printf("Usage: %s [options]\n", os.Args[0])
		fmt.Println(`If options are given, runs them and exit.`)
		fmt.Println(`Without options, runs jukebox.`)
		fmt.Println(`Options:`)
		flag.PrintDefaults()
	}
	flag.StringVar(&flagFile, `file`, ``, `Specify single audio file to play.`)
	flag.StringVar(&flagHashString, `hash`, ``, `Hash the given string.`)
	flag.BoolVar(&flagDefaultConfig, `default_config`, false, `Create configuration file from built-in configuration.`)
	flag.BoolVar(&flagUniOfficeTest, `test.v`, true, `Unioffice test.`)
}

func main() {
	flag.Parse()

	if flagDefaultConfig {
		err := cfg.save()
		if err != nil {
			fmt.Printf("%+v\n", err)
		}
		os.Exit(0)
	}

	if flagHashString != `` {
		s, err := hash(flagHashString)
		if err != nil {
			fmt.Printf("\"%s\" %+v\n", s, err)
		}
		os.Exit(0)
	}

	// Load configuration.
	err := cfg.load()
	if err != nil {
		fmt.Printf("%+v\n", err)
	}
	defer func() {
		err = cfg.save()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
		}
	}()

	// Configure logger.
	logger = NewLogger()
	go logger.log()
	defer logger.close()

	// Load translations.
	locale, err = NewLocale()
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	}

	// Create GPIO.
	gpio, err = NewGPIO()
	if err != nil {
		logger.fatalQueue <- fmt.Sprint(err)
	}

	// If there is a flagFile, play it and exit.
	// Else start the jukebox.
	if flagFile != `` {
		// jukebox.setVolume(jukebox.playListVolume)
		// jukebox.songToPlay <- flagFile
	} else {
		// Open internet radio database.
		internetRadio = NewInternetRadio()
		defer internetRadio.closeDB()

		// Start web administration.
		webAdmin = NewWebAdmin()
		webAdmin.action <- 'S'

		// Load lists.
		err = lists.load()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
		}
		if lists.artworkInit() {
			if err := lists.save(); err != nil {
				logger.queue <- fmt.Sprint(err)
			}
		}

		// Start user interface.
		userInterface = NewUserInterface()
		// Keyboard input goroutine.
		go userInterface.keyboard()

		// Random list goroutine.
		go lists.randomList()

		// If there is internet radio configured, play it.
		// Else begin with a random song.
		/*
			if jukebox.internetRadioSelected() {
				jukebox.setVolume(jukebox.internetRadioVolume)
				jukebox.songToPlay <- cfg.InternetRadioSelectedURL
				logger.queue <- fmt.Sprintf("playing internet radio station %s", cfg.InternetRadioSelectedName)
			} else {
				jukebox.setVolume(jukebox.randomListVolume)
				jukebox.songToPlay <- <-jukebox.randomListChannel
			}
		*/
	}

	jukebox.play()
}

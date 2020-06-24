package main

import (
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Logger structure.
type Logger struct {
	l                 *log.Logger
	fd                *os.File
	pc                []uintptr
	queue             chan string
	fatalQueue        chan string
	orderedSongsQueue chan string
	chipMoneyQueue    chan string
	rotate            chan bool
	reconfig          chan string
	gzDir             string
	orderedSongsFile  string
	chipMoneyFile     string
}

var logger *Logger

// NewLogger creates new Logger structure.
func NewLogger() *Logger {
	l := &Logger{
		pc:                make([]uintptr, 16),
		queue:             make(chan string, 2048),
		fatalQueue:        make(chan string),
		orderedSongsQueue: make(chan string, 2048),
		chipMoneyQueue:    make(chan string, 2048),
		rotate:            make(chan bool),
		reconfig:          make(chan string),
		gzDir:             `logs`,
		orderedSongsFile:  `ordered_songs.csv`,
		chipMoneyFile:     `chip_money.csv`,
	}

	l.open()
	l.l = log.New(l.fd, `jukebox on `, log.LstdFlags)

	return l
}

func (l *Logger) open() {
	var err error

	if cfg.LogFile == `` {
		l.fd = os.Stderr
	} else {
		if l.fd, err = os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664); err != nil {
			fmt.Print(err)
			l.fd = os.Stderr
		}
	}
}

func (l *Logger) close() {
	l.fd.Close()
}

func (l *Logger) checkRotate() bool {
	if l.fd != nil {
		stat, err := l.fd.Stat()
		if err != nil {
			return false
		}
		size := stat.Size()
		if size > 1000000 {
			return true
		}
	}
	return false
}

func (l *Logger) orderedSongsWrite(s string) error {
	fd, err := os.OpenFile(l.orderedSongsFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer fd.Close()
	writer := csv.NewWriter(fd)
	defer func() {
		if err == nil {
			writer.Flush()
			err = writer.Error()
		}
	}()
	return writer.Write([]string{fmt.Sprintf("%d", time.Now().Unix()), s})
}

func (l *Logger) chipMoneyWrite(s string) error {
	fd, err := os.OpenFile(l.chipMoneyFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer fd.Close()
	writer := csv.NewWriter(fd)
	defer func() {
		if err == nil {
			writer.Flush()
			err = writer.Error()
		}
	}()
	return writer.Write(append([]string{fmt.Sprintf("%d", time.Now().Unix())}, strings.Split(s, `,`)...))
}

func (l *Logger) log() {
	var (
		s  string
		b  bool
		r  bool
		rs string
	)

	for {
		s = ``
		b = false
		r = false
		rs = ``

		select {
		case s = <-l.orderedSongsQueue:
			if err := l.orderedSongsWrite(s); err != nil {
				l.l.Print(err)
			}
			continue
		case s = <-l.chipMoneyQueue:
			if err := l.chipMoneyWrite(s); err != nil {
				l.l.Print(err)
			}
			continue
		case s = <-l.fatalQueue:
			l.l.Fatal(s)
		case s = <-l.queue:
			b = l.checkRotate()
		case <-l.rotate:
			b = true
		case rs = <-l.reconfig:
			r = true
		}

		if b || r {
			// Rotate and change log file if requested
			err := func() error {
				var (
					err error
					fd  *os.File
					fn  string
				)

				if r {
					fn = rs
				} else {
					fn = cfg.LogFile
				}

				if _, err := os.Stat(l.gzDir); os.IsNotExist(err) {
					if err = os.MkdirAll(l.gzDir, 0755); err != nil {
						l.l.Print(err)
						l.gzDir = `.`
					}
				}
				zipFileName := fmt.Sprintf("%s/%s.%d.gz", l.gzDir, fn, time.Now().Unix())
				l.close()
				l.fd, err = os.Open(fn)
				if err != nil {
					return err
				}
				defer l.fd.Close()
				fd, err = os.Create(zipFileName)
				if err != nil {
					return err
				}
				defer fd.Close()
				zw := gzip.NewWriter(fd)
				defer zw.Close()
				_, err = io.Copy(zw, l.fd)
				return err
			}()
			if err != nil {
				fmt.Print(err)
			} else {
				if r {
					err = os.Remove(rs)
				} else {
					err = os.Truncate(cfg.LogFile, 0)
				}
				if err != nil {
					fmt.Print(err)
				}
			}
			l.open()
			l.l.SetOutput(l.fd)
		}

		if s != `` {
			// Log
			l.l.Print(s)
		}
	}
}

func (l *Logger) updateFromWebAdmin(r *http.Request) (msgOK, msgErr map[string][]interface{}) {
	var err error = nil
	msgOK = make(map[string][]interface{})
	msgErr = make(map[string][]interface{})
	newCf := cfg.copy()
	changed := false
	logRotate := false
	oldLogFile := cfg.LogFile
	logFile := strings.TrimSpace(r.FormValue(`logfile`))
	if logFile != cfg.LogFile {
		if logFile != `` {
			if m, _ := regexp.MatchString("^[[:word:]\x2D\x2E]+$", logFile); !m {
				msgErr[`Invalid log file name`] = []interface{}{template.HTMLEscapeString(logFile)}
			}
		}
		if _, ok := msgErr[`Invalid log file name`]; !ok {
			newCf.LogFile = logFile
			changed = true
			logRotate = true
		}
	}
	logfileRotateSize, err := strconv.Atoi(strings.TrimSpace(r.FormValue(`logfile_rotate_size`)))
	if err == nil {
		if logfileRotateSize != cfg.LogFileRotateSize {
			newCf.LogFileRotateSize = logfileRotateSize
			changed = true
		}
	} else {
		msgErr[`Invalid log file rotate size`] = []interface{}{template.HTMLEscapeString(r.FormValue(`logfile_rotate_size`))}
	}
	var debug byte = 1
	if r.FormValue(`debug`) == `` {
		debug = 0
	}
	if debug != cfg.Debug {
		newCf.Debug = debug
		changed = true
	}
	if changed {
		err = newCf.save()
		if err == nil {
			cfg = newCf.copy()
			if logRotate {
				logger.reconfig <- oldLogFile
			}
			msgOK[`Configuration changed`] = nil
		} else {
			logger.queue <- fmt.Sprint(err)
			msgErr[`Error saving data please try again`] = nil
		}
	}
	return
}

func (l *Logger) webAdminOrderedSongsList(r *http.Request) string {
	type kv struct {
		Key   string
		Value int
	}

	period := strings.TrimSpace(r.FormValue(`most_ordered_songs`))
	lp := len(period)

	if lp != 0 && lp != 23 {
		l.queue <- fmt.Sprintf("webAdminOrderedSongsList() invalid period '%s'", period)
		return ``
	}
	var s int64 = 0
	var e int64 = math.MaxInt64
	if lp > 0 {
		startTime, err := time.Parse(time.RFC3339, period[:10]+`T00:00:00Z`)
		if err != nil {
			l.queue <- fmt.Sprint(err)
			return ``
		}
		endTime, err := time.Parse(time.RFC3339, period[13:]+`T00:00:00Z`)
		if err != nil {
			l.queue <- fmt.Sprint(err)
			return ``
		}
		s = startTime.Unix()
		e = endTime.Unix()
	}

	fd, err := os.Open(l.orderedSongsFile)
	if err != nil {
		l.queue <- fmt.Sprint(err)
		return ``
	}
	defer fd.Close()
	reader := csv.NewReader(fd)
	if err != nil {
		l.queue <- fmt.Sprint(err)
		return ``
	}
	rvm := make(map[string]int)
	for {
		r, err := reader.Read()
		if err != nil {
			if err != io.EOF {
				l.queue <- fmt.Sprint(err)
			}
			break
		}
		if len(r) != 2 {
			l.queue <- fmt.Sprintf("webAdminOrderedSongsList() invalid row format %+v", r)
			break
		}
		t, err := strconv.ParseInt(r[0], 10, 64)
		if err != nil {
			l.queue <- fmt.Sprint(err)
			break
		}
		if t < s {
			continue
		}
		if t > e {
			break
		}
		if _, ok := rvm[r[1]]; ok {
			rvm[r[1]]++
		} else {
			rvm[r[1]] = 1
		}
	}

	if len(rvm) == 0 {
		return ``
	}
	rv := `<ul style="list-style-type:decimal;">`

	var ss []kv
	for k, v := range rvm {
		ss = append(ss, kv{k, v})
	}
	sort.SliceStable(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})
	for _, m := range ss {
		rv += fmt.Sprintf("<li>%s (%d)</li>", m.Key, m.Value)
	}

	return rv + `</ul>`
}

func (l *Logger) webAdminChipMoneyInsertedList(r *http.Request) string {
	period := strings.TrimSpace(r.FormValue(`chip_money_inserted`))
	lp := len(period)

	if lp != 0 && lp != 23 {
		l.queue <- fmt.Sprintf("webAdminChipMoneyInsertedList() invalid period '%s'", period)
		return ``
	}
	var s int64 = 0
	var e int64 = math.MaxInt64
	if lp > 0 {
		startTime, err := time.Parse(time.RFC3339, period[:10]+`T00:00:00Z`)
		if err != nil {
			l.queue <- fmt.Sprint(err)
			return ``
		}
		endTime, err := time.Parse(time.RFC3339, period[13:]+`T00:00:00Z`)
		if err != nil {
			l.queue <- fmt.Sprint(err)
			return ``
		}
		s = startTime.Unix()
		e = endTime.Unix()
	}

	fd, err := os.Open(l.chipMoneyFile)
	if err != nil {
		l.queue <- fmt.Sprint(err)
		return ``
	}
	defer fd.Close()
	reader := csv.NewReader(fd)
	if err != nil {
		l.queue <- fmt.Sprint(err)
		return ``
	}
	rvm := make(map[string]int64)
	for {
		r, err := reader.Read()
		if err != nil {
			if err != io.EOF {
				l.queue <- fmt.Sprint(err)
			}
			break
		}
		if len(r) != 3 {
			l.queue <- fmt.Sprintf("webAdminChipMoneyInsertedList() invalid row format %+v", r)
			break
		}
		if _, ok := cfg.selectionSourceTypes[r[1]]; !ok {
			l.queue <- fmt.Sprintf("webAdminChipMoneyInsertedList() unknown selection source %+v", r)
			break
		}
		a, err := strconv.ParseInt(r[2], 10, 64)
		if err != nil {
			l.queue <- fmt.Sprint(err)
			break
		}
		t, err := strconv.ParseInt(r[0], 10, 64)
		if err != nil {
			l.queue <- fmt.Sprint(err)
			break
		}
		if t < s {
			continue
		}
		if t > e {
			break
		}
		if _, ok := rvm[r[1]]; ok {
			rvm[r[1]] += a
		} else {
			rvm[r[1]] = a
		}
	}

	if len(rvm) == 0 {
		return ``
	}
	rv := ``
	for k, v := range rvm {
		rv += fmt.Sprintf("%s: %d<br>", locale.GetD(`config`, cfg.selectionSourceTypes[k]), v)
	}

	return rv
}

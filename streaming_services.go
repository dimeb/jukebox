package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bvinc/go-sqlite-lite/sqlite3"
)

type StreamingServices struct {
	dbName       string
	dbh          *sqlite3.Conn
	mux          sync.Mutex
	openDuration time.Duration
	tickerOpen   *time.Ticker
}

var streamingServices *StreamingServices

// NewStreamingServices creates new Streaming structure.
func NewStreamingServices() *StreamingServices {
	ss := &StreamingServices{
		dbName:       `streaming.db`,
		openDuration: 30 * time.Second,
	}

	go ss.open()

	return ss
}

// Check tables.
func (ss *StreamingServices) checkTables(dbh *sqlite3.Conn) error {
	if !ss.opened() {
		return errors.New(`streaming database is closed`)
	}

	return dbh.WithTx(func() error {
		return dbh.Exec(`CREATE TABLE IF NOT EXISTS playlist (` +
			`playlist_id INTEGER PRIMARY KEY,` +
			`origin TEXT,` +
			`name TEXT,` +
			`id TEXT,` +
			`url TEXT,` +
			`tracks_url TEXT,` +
			`tracks_total NTEGER NOT NULL,` +
			`image_url TEXT,` +
			`image_height NTEGER NOT NULL,` +
			`image_width NTEGER NOT NULL` +
			`) WITHOUT ROWID;` +
			`CREATE INDEX IF NOT EXISTS playlist_origin ON playlist (origin);` +
			`CREATE INDEX IF NOT EXISTS playlist_name ON playlist (name);` +
			`CREATE TABLE IF NOT EXISTS track (` +
			`track_id INTEGER PRIMARY KEY,` +
			`playlist_id INTEGER NOT NULL,` +
			`origin TEXT,` +
			`name TEXT,` +
			`artist TEXT,` +
			`id TEXT,` +
			`url TEXT,` +
			`image_url TEXT,` +
			`image_height NTEGER NOT NULL,` +
			`image_width NTEGER NOT NULL` +
			`FOREIGN KEY(playlist_id) REFERENCES playlist(playlist_id)` +
			`) WITHOUT ROWID;` +
			`CREATE INDEX IF NOT EXISTS track_origin ON playlist (origin);` +
			`CREATE INDEX IF NOT EXISTS track_name ON playlist (name);`)
	})
}

func (ss *StreamingServices) openDB() bool {
	var err error

	// Check database file.
	openFlags := sqlite3.OPEN_READWRITE
	info, err := os.Stat(ss.dbName)
	if os.IsNotExist(err) {
		openFlags |= sqlite3.OPEN_CREATE
	} else {
		if info.IsDir() {
			logger.queue <- fmt.Sprintf("streaming database name \"%s\": there is directory with same name", ss.dbName)
			// Unconditionally delete directory if empty.
			if err = os.Remove(ss.dbName); err != nil {
				logger.queue <- fmt.Sprint(err)
				return false
			}
			openFlags |= sqlite3.OPEN_CREATE
		}
	}

	// Open database.
	ss.mux.Lock()
	ss.dbh, err = sqlite3.Open(ss.dbName, openFlags)
	ss.mux.Unlock()
	if err != nil {
		logger.queue <- fmt.Sprintf("%+v \"%s\"", err, ss.dbName)
		return false
	}
	ss.dbh.BusyTimeout(5 * time.Second)

	// Check integrity.
	if openFlags&sqlite3.OPEN_CREATE == 0 {
		stmt, err := ss.dbh.Prepare(`PRAGMA integrity_check`)
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return false
		}
		row, err := stmt.Step()
		if err == nil {
			if row {
				s := ``
				err = stmt.Scan(&s)
				if err == nil {
					if strings.ToLower(s) != `ok` {
						err = fmt.Errorf(`database integrity check failed`)
					}
				}
			} else {
				err = fmt.Errorf(`no result from integrity_check, failed`)
			}
		}
		stmt.Close()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			ss.closeDB()
			// Remove database file and open it again.
			if err = os.Remove(ss.dbName); err != nil {
				logger.queue <- fmt.Sprint(err)
				return false
			}
			logger.queue <- `recreating database`
			openFlags |= sqlite3.OPEN_CREATE
			ss.mux.Lock()
			ss.dbh, err = sqlite3.Open(ss.dbName, openFlags)
			ss.mux.Unlock()
			if err != nil {
				logger.queue <- fmt.Sprintf("%+v \"%s\"", err, ss.dbName)
				return false
			}
			ss.dbh.BusyTimeout(5 * time.Second)
		}
	}

	err = ss.checkTables(ss.dbh)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return false
	}

	return true
}

func (ss *StreamingServices) closeDB() {
	if !ss.opened() {
		return
	}
	if err := ss.dbh.Close(); err != nil {
		logger.queue <- fmt.Sprint(err)
	}
	ss.mux.Lock()
	ss.dbh = nil
	ss.mux.Unlock()
}

func (ss *StreamingServices) open() {
	ss.tickerOpen = time.NewTicker(ss.openDuration)
	defer ss.tickerOpen.Stop()
	for ; true; <-ss.tickerOpen.C {
		if ss.openDB() {
			break
		}
		logger.queue <- `st.openDB() error`
		ss.closeDB()
	}
}

func (ss *StreamingServices) opened() bool {
	return ss.dbh != nil
}

// Updates configuration with values from web admin page.
func (ss *StreamingServices) updateFromWebAdmin(r *http.Request) (msgOK, msgErr map[string][]interface{}) {
	var err error = nil
	msgOK = make(map[string][]interface{})
	msgErr = make(map[string][]interface{})
	newCfg := cfg.copy()
	changed := false
	deleteFromDatabase := []string{}
	for k, v := range cfg.StreamingServices {
		for v1, v2 := range v {
			val := strings.TrimSpace(r.FormValue(k + `_` + v1))
			if v2 != val {
				if v1 == `active` && val == `` {
					val = `0`
				}
				newCfg.StreamingServices[k][v1] = val
				changed = true
				if newCfg.StreamingServices[k][`active`] == `0` {
					deleteFromDatabase = append(deleteFromDatabase, k)
				}
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
	if len(deleteFromDatabase) > 0 && ss.opened() {
		cond := `('` + strings.Join(deleteFromDatabase, `','`) + `')`
		stmtp, err := ss.dbh.Prepare(`SELECT url FROM playlist WHERE origin IN `+cond, 0)
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return
		}
		defer stmtp.Close()
		url := ``
		urls := make(map[string]byte)
		row := false
		for {
			row, err = stmtp.Step()
			if err != nil {
				logger.queue <- fmt.Sprint(err)
				return
			}
			if !row {
				break
			}
			err = stmtp.Scan(&url)
			if err != nil {
				logger.queue <- fmt.Sprint(err)
				return
			}
			urls[url] = '1'
		}
		stmtt, err := ss.dbh.Prepare(`SELECT url FROM track WHERE playlist_id IN (SELECT playlist_id FROM playlist WHERE origin IN `+cond+`)`, 0)
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return
		}
		defer stmtt.Close()
		for {
			row, err = stmtt.Step()
			if err != nil {
				logger.queue <- fmt.Sprint(err)
				return
			}
			if !row {
				break
			}
			err = stmtt.Scan(&url)
			if err != nil {
				logger.queue <- fmt.Sprint(err)
				return
			}
			urls[url] = '1'
		}
		err = ss.dbh.Exec(`DELETE FROM playlist WHERE origin IN ` + cond)
		if err != nil {
			logger.queue <- fmt.Sprint(err)
		}
		rl := []string{}
		for _, v := range lists.RandomList {
			if _, ok := urls[v]; !ok {
				rl = append(rl, v)
			}
		}
		playListChanged := false
		randomListChanged := len(rl) != len(lists.RandomList)
		if playListChanged || randomListChanged {
			newL := lists.copy()
			newL.RandomList = append(newL.RandomList, rl...)
			err = newL.save()
			if err == nil {
				err = lists.load()
				if err == nil {
					if randomListChanged {
						lists.randomList()
					}
					if playListChanged {
						if userInterface.wsCanWrite {
							userInterface.screenMessageChannel <- `init`
						}
					}
				}
			}
			if err != nil {
				logger.queue <- fmt.Sprint(err)
			}
		}
	}
	return
}

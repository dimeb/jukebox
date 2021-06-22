package main

import (
	"errors"
	"fmt"
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
	if ss.dbh == nil {
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

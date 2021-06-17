package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bvinc/go-sqlite-lite/sqlite3"
)

// InternetRadio internet radio structure
type InternetRadio struct {
	dbSQLName      string
	dbName         string
	dbDownloadName string
	dbh            *sqlite3.Conn
	mux            sync.Mutex
	openDuration   time.Duration
	tickerOpen     *time.Ticker
	tickerUpdate   *time.Ticker
	updateChannel  chan byte
	updateRegular  byte
	updateForced   byte
	updating       bool
}

var internetRadio *InternetRadio

// NewInternetRadio creates new internet radio structure.
func NewInternetRadio() *InternetRadio {
	ir := &InternetRadio{
		dbSQLName:      `internet_radio.sql.gz`,
		dbName:         `internet_radio.db`,
		dbDownloadName: `internet_radio_download.db`,
		openDuration:   30 * time.Second,
		tickerUpdate:   time.NewTicker(24 * time.Hour),
		updateChannel:  make(chan byte, 1),
		updateRegular:  1,
		updateForced:   2,
	}

	go func() {
		ir.open()
		go ir.updateDB()
		ir.updateChannel <- ir.updateRegular
	}()

	return ir
}

// Check if radio URL is reachable.
func (ir *InternetRadio) reachable() bool {
	u, err := url.ParseRequestURI(cfg.InternetRadioSelectedURL)
	if err != nil {
		logger.queue <- fmt.Sprintf("Invalid internet radio URL: %s", cfg.InternetRadioSelectedURL)
		return false
	}
	_, err = net.DialTimeout(`tcp`, u.Host, 3*time.Second)
	if err != nil {
		logger.queue <- fmt.Sprintf("Internet radio %s unreachable: %v", cfg.InternetRadioSelectedURL, err)
		return false
	}
	return true
}

// Check tables.
func (ir *InternetRadio) checkTables(dbh *sqlite3.Conn) error {
	return dbh.WithTx(func() error {
		return dbh.Exec(`CREATE TABLE IF NOT EXISTS station (` +
			`station_id INTEGER PRIMARY KEY,` +
			`name TEXT,` +
			`url TEXT,` +
			`homepage TEXT,` +
			`favicon TEXT,` +
			`created_at DATETIME NOT NULL,` +
			`country TEXT,` +
			`language TEXT,` +
			`tags TEXT,` +
			`votes NTEGER NOT NULL,` +
			`subcountry TEXT,` +
			`click_count INTEGER NOT NULL,` +
			`click_trend INTEGER NOT NULL,` +
			`click_timestamp DATETIME,` +
			`codec TEXT,` +
			`last_check_ok INTEGER NOT NULL,` +
			`last_check_time DATETIME,` +
			`bitrate INTEGER NOT NULL,` +
			`url_cache TEXT NOT NULL,` +
			`last_check_ok_time DATETIME,` +
			`hls INTEGER NOT NULL,` +
			`change_uuid TEXT UNIQUE,` +
			`station_uuid TEXT UNIQUE,` +
			`country_code TEXT,` +
			`last_local_check_time DATETIME,` +
			`country_subdivision_code TEXT,` +
			`geo_lat DOUBLE,` +
			`geo_long DOUBLE,` +
			`ssl_error INTEGER NOT NULL,` +
			`language_codes TEXT);` +
			`CREATE INDEX IF NOT EXISTS station_name ON station (name);` +
			`CREATE INDEX IF NOT EXISTS station_country ON station (country);` +
			`CREATE INDEX IF NOT EXISTS station_tags ON station (tags);` +
			`CREATE TABLE IF NOT EXISTS tag (` +
			`tag_name TEXT PRIMARY KEY,` +
			`station_count INTEGER,` +
			`station_count_working INTEGER` +
			`) WITHOUT ROWID;` +
			`CREATE TABLE IF NOT EXISTS language (` +
			`language_name TEXT PRIMARY KEY,` +
			`station_count INTEGER,` +
			`station_count_working INTEGER` +
			`) WITHOUT ROWID;`)
	})
}

func (ir *InternetRadio) openDB() bool {
	var err error

	// Check database file.
	openFlags := sqlite3.OPEN_READWRITE
	info, err := os.Stat(ir.dbName)
	if os.IsNotExist(err) {
		openFlags |= sqlite3.OPEN_CREATE
	} else {
		if info.IsDir() {
			logger.queue <- fmt.Sprintf("internet radio database name \"%s\": there is directory with same name", ir.dbName)
			// Unconditionally delete directory if empty.
			if err = os.Remove(ir.dbName); err != nil {
				logger.queue <- fmt.Sprint(err)
				return false
			}
			openFlags |= sqlite3.OPEN_CREATE
		}
	}

	// Open database.
	ir.mux.Lock()
	ir.dbh, err = sqlite3.Open(ir.dbName, openFlags)
	ir.mux.Unlock()
	if err != nil {
		logger.queue <- fmt.Sprintf("%+v \"%s\"", err, ir.dbName)
		return false
	}
	ir.dbh.BusyTimeout(5 * time.Second)

	// Check integrity.
	if openFlags&sqlite3.OPEN_CREATE == 0 {
		stmt, err := ir.dbh.Prepare(`PRAGMA integrity_check`)
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
			ir.closeDB()
			// Remove database file and open it again.
			if err = os.Remove(ir.dbName); err != nil {
				logger.queue <- fmt.Sprint(err)
				return false
			}
			logger.queue <- `recreating database`
			openFlags |= sqlite3.OPEN_CREATE
			ir.mux.Lock()
			ir.dbh, err = sqlite3.Open(ir.dbName, openFlags)
			ir.mux.Unlock()
			if err != nil {
				logger.queue <- fmt.Sprintf("%+v \"%s\"", err, ir.dbName)
				return false
			}
			ir.dbh.BusyTimeout(5 * time.Second)
		}
	}

	err = ir.checkTables(ir.dbh)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return false
	}

	return true
}

func (ir *InternetRadio) closeDB() {
	if !ir.opened() {
		return
	}
	if err := ir.dbh.Close(); err != nil {
		logger.queue <- fmt.Sprint(err)
	}
	ir.mux.Lock()
	ir.dbh = nil
	ir.mux.Unlock()
}

func (ir *InternetRadio) open() {
	ir.tickerOpen = time.NewTicker(ir.openDuration)
	defer ir.tickerOpen.Stop()
	for ; true; <-ir.tickerOpen.C {
		if ir.openDB() {
			break
		}
		logger.queue <- `ir.openDB() error`
		ir.closeDB()
	}
}

func (ir *InternetRadio) opened() bool {
	return ir.dbh != nil
}

func (ir *InternetRadio) escape(s string) string {
	if strings.Index(s, `''`) >= 0 {
		return s
	} else if strings.Index(s, `\`) >= 0 {
		return strings.ReplaceAll(s, `\'`, `''`)
	}
	return strings.ReplaceAll(s, `'`, `''`)
}

func (ir *InternetRadio) updateDB() {
	var (
		err  error
		t    byte
		info os.FileInfo
	)

	layout := `20060102`
	defer ir.tickerUpdate.Stop()
	for {
		ir.updating = false
		os.Remove(ir.dbDownloadName)

		select {
		case <-ir.tickerUpdate.C:
			t = ir.updateRegular
		case t = <-ir.updateChannel:
		}

		if !ir.opened() {
			logger.queue <- `updateDB: nil internet radio database handler`
			continue
		}

		// For ir.updateRegular action,
		// when ir.dbSQLName exists and is from today, skip.
		info, err = os.Stat(ir.dbSQLName)
		if !os.IsNotExist(err) {
			if info.IsDir() {
				logger.queue <- fmt.Sprintf("internet radio download file name \"%s\": there is directory with same name", ir.dbSQLName)
				if err = os.Remove(ir.dbSQLName); err != nil {
					logger.queue <- fmt.Sprint(err)
					continue
				}
			} else {
				if t == ir.updateRegular && info.ModTime().Format(layout) == time.Now().Format(layout) {
					continue
				}
			}
		}

		// Download internet radio database and update local database.
		ir.updating = true
		logger.queue <- `internet radio database update started`
		err = func() error {
			fd, er := os.Create(ir.dbSQLName)
			if er != nil {
				return er
			}
			defer fd.Close()
			logger.queue <- `downloading internet radio database`
			resp, er := http.Get(cfg.InternetRadioDownloadURL)
			if er != nil {
				return er
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("error downloading %s , status code %v", cfg.InternetRadioDownloadURL, resp.StatusCode)
			}
			_, er = io.Copy(fd, resp.Body)
			if er != nil {
				return er
			}
			_, er = fd.Seek(0, 0)
			if er != nil {
				return er
			}
			gz, er := gzip.NewReader(fd)
			if er != nil {
				return er
			}
			defer gz.Close()

			dbh, er := sqlite3.Open(ir.dbDownloadName, sqlite3.OPEN_READWRITE|sqlite3.OPEN_CREATE)
			if er != nil {
				return er
			}
			defer dbh.Close()
			ir.checkTables(dbh)

			logger.queue <- `updating local internet radio database`
			er = dbh.WithTx(func() error {
				reader := bufio.NewReader(gz)
				for {
					line, e := reader.ReadString('\n')
					if e != nil {
						if e == io.EOF {
							break
						} else {
							return e
						}
					}

					sp := "INSERT INTO `Station` VALUES "
					tp := "INSERT INTO `TagCache` VALUES "
					lp := "INSERT INTO `LanguageCache` VALUES "
					p := ``
					if strings.HasPrefix(line, sp) {
						line = line[len(sp):]
						p = `INSERT INTO station VALUES `
					} else if strings.HasPrefix(line, tp) {
						line = line[len(tp):]
						p = `INSERT INTO tag VALUES `
					} else if strings.HasPrefix(line, lp) {
						line = line[len(lp):]
						p = `INSERT INTO language VALUES `
					} else {
						continue
					}
					aline := strings.Split(line, `),(`)
					line = ``
					for i, v := range aline {
						if i == 0 {
							v += `)`
						} else if i == len(aline)-1 {
							v = `(` + v[:len(v)-2]
						} else {
							v = `(` + v + `)`
						}
						v = strings.ReplaceAll(v, `\'`, `''`)
						if e := dbh.Exec(p + v); e != nil {
							logger.queue <- fmt.Sprintf("%v : %s", e, p+v)
						}
					}
				}
				return dbh.Exec(`UPDATE station SET language=lower(language),country_code=upper(country_code);`)
			})

			return er
		}()
		if err == nil {
			ir.closeDB()
			err = os.Rename(ir.dbDownloadName, ir.dbName)
			ir.open()
		}
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			logger.queue <- `error during database update`
		}
		logger.queue <- `internet radio database update finished`
	}
}

// Initial web admin page data.
func (ir *InternetRadio) webAdminPage() (language map[string]int, tag []string, country map[string]string, ok bool) {
	var err error

	language = make(map[string]int)
	tag = []string{}
	country = map[string]string{`--`: ``}

	if ok = ir.opened(); !ok {
		return
	}
	if ir.updating {
		ok = false
		return
	}
	ok = false

	// Languages
	stmt, err := ir.dbh.Prepare(`SELECT language_name, station_count_working FROM language WHERE station_count_working > ? ORDER BY 1`, 0)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return
	}
	defer stmt.Close()
	for {
		row, err := stmt.Step()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return
		}
		if !row {
			break
		}
		var name string
		var cnt int
		err = stmt.Scan(&name, &cnt)
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return
		}
		language[name] = cnt
	}

	// Tags
	stmt, err = ir.dbh.Prepare(`SELECT tag_name, station_count_working FROM tag WHERE station_count_working > ? ORDER BY 2 DESC, 1 ASC`, 0)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return
	}
	defer stmt.Close()
	for {
		row, err := stmt.Step()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return
		}
		if !row {
			break
		}
		var name string
		var cnt int
		err = stmt.Scan(&name, &cnt)
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return
		}
		name = strings.TrimSpace(name)
		tag = append(tag, fmt.Sprintf("%s (%d)", name, cnt))
	}

	// Countries
	stmt, err = ir.dbh.Prepare(`SELECT country_code, country FROM station WHERE length(trim(country_code)) > ? GROUP BY 1 ORDER BY 1`, 0)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return
	}
	defer stmt.Close()
	for {
		row, err := stmt.Step()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return
		}
		if !row {
			break
		}
		var code string
		var name string
		err = stmt.Scan(&code, &name)
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return
		}
		country[code] = name
	}
	ok = true

	return
}

// AJAX call query.
func (ir *InternetRadio) webAdminData(r *http.Request) ([]byte, bool) {
	type item struct {
		ID     string  `json:"id"`
		Name   string  `json:"name"`
		File   string  `json:"file"`
		Parent *string `json:"parent"`
	}

	var (
		data      []item
		emptyData []byte = []byte(`[]`)
	)

	if !ir.opened() {
		return emptyData, false
	}
	if ir.updating {
		return emptyData, false
	}

	name := strings.TrimSpace(r.FormValue(`search_name`))
	country := r.Form[`search_country`]
	tag := r.Form[`search_tag`]
	language := r.Form[`search_language`]

	query := `SELECT name, url, url_cache, language, tags, country, country_code FROM station WHERE last_check_ok = 1`
	if name != `` {
		query += ` AND name  LIKE '%` + ir.escape(name) + `%'`
	}
	if len(country) > 0 {
		for i := range country {
			if country[i] == `--` {
				country[i] = ``
			} else {
				country[i] = ir.escape(country[i])
			}
		}
		query += ` AND country_code  IN ('` + strings.Join(country, `','`) + `')`
	}
	if len(tag) > 0 {
		for i := range tag {
			tag[i] = ir.escape(tag[i])
		}
		query += ` AND (tags LIKE '%` + strings.Join(tag, `%' OR tags LIKE '%`) + `%')`
	}
	if len(language) > 0 {
		for i := range language {
			language[i] = ir.escape(language[i])
		}
		query += ` AND language  IN ('` + strings.Join(language, `','`) + `')`
	}
	query += ` ORDER BY country_code, name`

	// logger.queue <- query
	stmt, err := ir.dbh.Prepare(query)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return emptyData, false
	}
	defer stmt.Close()
	var i int = 1
	var cc string
	var cci int
	for {
		row, err := stmt.Step()
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return emptyData, false
		}
		if !row {
			break
		}
		var name string
		var url string
		var urlCache string
		var language string
		var tags string
		var country string
		var countryCode string
		err = stmt.Scan(&name, &url, &urlCache, &language, &tags, &country, &countryCode)
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			// return emptyData, false
			continue
		}
		if strings.TrimSpace(countryCode) == `` {
			countryCode = `--`
		}
		country = strings.TrimSpace(country)
		if country != `` {
			country = ` - ` + country
		}
		if cc != countryCode {
			data = append(data, item{strconv.Itoa(i), countryCode + country, ``, nil})
			cc = countryCode
			cci = i
			i++
		}
		ccip := strconv.Itoa(cci)
		if strings.TrimSpace(language) != `` {
			name += ` - ` + language
		}
		if strings.TrimSpace(tags) != `` {
			name += ` (` + tags + `)`
		}
		if len(urlCache) > 0 {
			url = urlCache
		}
		data = append(data, item{strconv.Itoa(i), name, url, &ccip})
		i++
	}

	if len(data) == 0 {
		return emptyData, true
	}

	j, err := json.Marshal(&data)
	if err != nil {
		return emptyData, false
	}

	return j, true
}

// Updates configuration with values from web admin page.
func (ir *InternetRadio) updateFromWebAdmin(r *http.Request) (msgOK, msgErr map[string][]interface{}) {
	var err error = nil
	msgOK = make(map[string][]interface{})
	msgErr = make(map[string][]interface{})
	newCfg := cfg.copy()
	changed := false
	internetRadioDownloadURL := strings.TrimSpace(r.FormValue(`download_url`))
	if internetRadioDownloadURL != cfg.InternetRadioDownloadURL && internetRadioDownloadURL != `` {
		if _, err := url.ParseRequestURI(internetRadioDownloadURL); err != nil {
			logger.queue <- fmt.Sprint(err)
			msgErr[`Invalid download URL`] = []interface{}{template.HTMLEscapeString(internetRadioDownloadURL)}
		} else {
			newCfg.InternetRadioDownloadURL = internetRadioDownloadURL
			changed = true
		}
	}
	selectedRadioChange := false
	toDelete := strings.TrimSpace(r.FormValue(`selected_url_delete`))
	if toDelete == `1` {
		if cfg.BackgroundMusic == `internet radio` {
			msgErr[`Cannot delete selected station`] = nil
		} else {
			newCfg.InternetRadioSelectedURL = ``
			newCfg.InternetRadioSelectedName = ``
			changed = true
		}
	} else {
		internetRadioSelectedName := strings.TrimSpace(r.FormValue(`selected_name`))
		internetRadioSelectedURL := strings.TrimSpace(r.FormValue(`selected_url`))
		if internetRadioSelectedURL != cfg.InternetRadioSelectedURL {
			newCfg.InternetRadioSelectedURL = internetRadioSelectedURL
			newCfg.InternetRadioSelectedName = internetRadioSelectedName
			changed = true
			selectedRadioChange = true
		}
	}
	if changed {
		err = newCfg.save()
		if err == nil {
			cfg = newCfg.copy()
			if selectedRadioChange && cfg.BackgroundMusic == `internet radio` {
				jukebox.internetRadioChanged <- true
				logger.queue <- fmt.Sprintf("internet radio station selected %s", cfg.InternetRadioSelectedName)
			}
			msgOK[`Configuration changed`] = nil
		} else {
			logger.queue <- fmt.Sprint(err)
			msgErr[`Error saving data please try again`] = nil
		}
	}
	return
}

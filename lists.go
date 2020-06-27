package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
	"github.com/disintegration/imaging"
	"gopkg.in/yaml.v2"
)

// Song structure.
type Song struct {
	File   string `yaml:"file"`
	Name   string `yaml:"name"`
	Author string `yaml:"author"`
	Icon   string `yaml:"icon,omitempty"`
}

// Lists song lists structure.
type Lists struct {
	// RandomList folders and songs for the random list.
	RandomList []string `yaml:"random_list,omitempty"`
	// PlayList play list.
	PlayList map[string]map[string]Song `yaml:"play_list"`
	// ShowPlayList show or not play list on screen.
	ShowPlayList map[string]bool `yaml:"show_play_list"`
	// BrowseList folder list for browse skin.
	BrowseList []string `yaml:"browse_list,omitempty"`
	// Position of song name and author in label.
	LabelContent string `yaml:"label_content"`
	// Label width.
	labelWidth int
	// Label height.
	labelHeight int
	// Music root directory.
	rootDir string
	// Lists file.
	listsFile string
	// Number of songs per slot.
	playListSongsPerSlot [24]string
	// Play list number.
	playListNumber string
	// Valid song codecs.
	codecs []string
	// Options for label content.
	labelContentOptions [18]string
	// Artwork directory
	artworkDir string
	// random play list file
	randomPlayListFile string
}

var (
	lists = &Lists{
		RandomList: []string{},
		PlayList:   make(map[string]map[string]Song),
		ShowPlayList: map[string]bool{
			`0`: true,
			`1`: true,
			`2`: true,
			`3`: true,
			`4`: true,
			`5`: false,
			`6`: false,
			`7`: false,
			`8`: false,
			`9`: false,
		},
		BrowseList:   []string{},
		LabelContent: `name-left-author-left`,
		labelWidth:   78,
		labelHeight:  26,
		rootDir:      `Music/`,
		listsFile:    `lists.yaml`,
		playListSongsPerSlot: [24]string{
			`a`, `b`, `c`, `d`, `e`, `f`,
			`g`, `h`, `i`, `j`, `k`, `l`,
			`m`, `n`, `o`, `p`, `q`, `r`,
			`s`, `t`, `u`, `v`, `w`, `x`,
		},
		playListNumber: `0`,
		codecs:         []string{`flac`, `mp3`, `ogg`, `wav`, `wma`},
		labelContentOptions: [18]string{
			`name-left-author-left`,
			`name-left-author-center`,
			`name-left-author-right`,
			`name-center-author-left`,
			`name-center-author-center`,
			`name-center-author-right`,
			`name-right-author-left`,
			`name-right-author-center`,
			`name-right-author-right`,
			`author-left-name-left`,
			`author-left-name-center`,
			`author-left-name-right`,
			`author-center-name-left`,
			`author-center-name-center`,
			`author-center-name-right`,
			`author-right-name-left`,
			`author-right-name-center`,
			`author-right-name-right`,
		},
		artworkDir:         `art/`,
		randomPlayListFile: `random_list.m3u8`,
	}
)

// Load random and play lists from yaml file.
func (l *Lists) load() error {
	toSave := false
	_, err := os.Stat(l.listsFile)
	if os.IsNotExist(err) {
		toSave = true
	} else {
		lf, err := ioutil.ReadFile(l.listsFile)
		if err == nil {
			err = yaml.Unmarshal(lf, &l)
		}
		if err != nil {
			toSave = true
			logger.queue <- fmt.Sprint(err)
		}
	}
	// Check and optionally recreate play list as defined.
	ok := false
	pl := make(map[string]map[string]Song)
	for sl := range l.ShowPlayList {
		pl[sl] = make(map[string]Song)
		for _, s := range l.playListSongsPerSlot {
			if _, ok = l.PlayList[sl]; ok {
				if v, ok := l.PlayList[sl][s]; ok {
					pl[sl][s] = v
				}
			}
			if !ok {
				pl[sl][s] = Song{}
				toSave = true
			}
		}
	}
	l.copyPlayList(&Lists{PlayList: pl})

	if toSave {
		err = l.save()
	}

	return err
}

// Saves lists in cfg.ListsFile.
func (l Lists) save() error {
	data, err := yaml.Marshal(l)
	if err == nil {
		err = ioutil.WriteFile(l.listsFile, data, 0644)
	}
	return err
}

// Returns a new Lists object, copy of the current Lists object.
func (l Lists) copy() Lists {
	var newL = Lists{
		LabelContent:   l.LabelContent,
		rootDir:        l.rootDir,
		listsFile:      l.listsFile,
		playListNumber: l.playListNumber,
	}

	newL.RandomList = append(newL.RandomList, l.RandomList...)
	newL.BrowseList = append(newL.BrowseList, l.BrowseList...)
	newL.labelWidth = l.labelWidth
	newL.labelHeight = l.labelHeight
	newL.codecs = l.codecs
	newL.labelContentOptions = l.labelContentOptions
	newL.playListSongsPerSlot = l.playListSongsPerSlot

	newL.ShowPlayList = make(map[string]bool)
	for k, v := range l.ShowPlayList {
		newL.ShowPlayList[k] = v
	}

	newL.PlayList = make(map[string]map[string]Song)
	newL.copyPlayList(&l)

	return newL
}

// Copy play list in receiver.
func (l *Lists) copyPlayList(src *Lists) {
	for k, v := range src.PlayList {
		m := make(map[string]Song)
		for k1, v1 := range v {
			m[k1] = v1
		}
		l.PlayList[k] = m
	}
}

// Check song's codec.
func (l Lists) checkSong(name string) bool {
	for _, codec := range l.codecs {
		if strings.HasSuffix(name, `.`+codec) || strings.HasSuffix(name, `.`+strings.ToUpper(codec)) {
			return true
		}
	}
	// logger.queue <- fmt.Sprintf("invalid song format %s", name)
	return false
}

// Generate random song list.
func (l Lists) randomList() {
	var lst []string
	if len(l.RandomList) > 0 {
		copy(lst, l.RandomList)
	} else {
		files, _ := ioutil.ReadDir(l.rootDir)
		for _, file := range files {
			lst = append(lst, file.Name())
		}
	}
	if len(lst) == 0 {
		return
	}
	f, err := os.Create(l.randomPlayListFile)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return
	}
	defer f.Close()
	for _, dir := range lst {
		dir = l.rootDir + dir
		info, err := os.Stat(dir)
		if err != nil {
			logger.queue <- fmt.Sprint(err)
			continue
		}
		if info.Mode().IsRegular() {
			if l.checkSong(dir) {
				_, err = f.Write([]byte(dir + "\n"))
				if err != nil {
					logger.queue <- fmt.Sprint(err)
				}
			}
			continue
		}
		err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return filepath.SkipDir
			}
			if info.Mode().IsRegular() {
				if l.checkSong(path) {
					if _, e := f.Write([]byte(path + "\n")); e != nil {
						logger.queue <- fmt.Sprint(e)
					}
				}
			}
			return nil
		})
		if err != nil {
			logger.queue <- fmt.Sprint(err)
		}
	}
}

// Finds song from PlayList.
func (l *Lists) chosenSongOrList(keyCode []byte, playListNumber ...string) (string, bool, error) {
	for k := range l.PlayList {
		if bytes.HasSuffix(keyCode, []byte(k)) {
			l.playListNumber = k
			return ``, true, nil
		}
	}
	var pl string
	if len(playListNumber) > 0 {
		pl = playListNumber[0]
	} else {
		pl = l.playListNumber
	}
	lst, ok := l.PlayList[pl]
	if !ok {
		return ``, false, fmt.Errorf("list %s doesn't exist,", pl)
	}
	ln := len(keyCode)
	if ln <= 0 {
		return ``, false, fmt.Errorf(`empty key value from keyboard`)
	}
	s, ok := lst[string(keyCode)]
	if !ok {
		n := strings.LastIndexByte(string(keyCode), 27)
		if n < 0 {
			keyCode = []byte{keyCode[ln-1]}
		} else {
			keyCode = keyCode[n : ln-1]
		}
		s, ok = lst[string(keyCode)]
	}
	if ok {
		fileName := l.rootDir + s.File
		if s.File != `` && l.checkSong(fileName) {
			return fileName, false, nil
		}
		return ``, false, fmt.Errorf("invalid selection")
	}
	return ``, false, fmt.Errorf("key %v does not exist", keyCode)
}

// Updates lists with values from web admin page.
func (l *Lists) updateFromWebAdmin(r *http.Request) (msgOK, msgErr map[string][]interface{}) {
	var err error
	msgOK = make(map[string][]interface{})
	msgErr = make(map[string][]interface{})
	newL := l.copy()
	changed := false
	randomListChanged := false
	playListChanged := false
	browseListChanged := false
	if len(r.Form[`random_list`]) > 0 {
		newL.RandomList = append([]string{}, r.Form[`random_list`]...)
		if strings.Join(newL.RandomList, ``) != strings.Join(l.RandomList, ``) {
			changed = true
			randomListChanged = true
		}
	} else {
		newL.RandomList = []string{}
	}
	for sl := range newL.ShowPlayList {
		newL.ShowPlayList[sl] = false
	}
	if len(r.Form[`show_play_list`]) > 0 {
		for _, sl := range r.Form[`show_play_list`] {
			newL.ShowPlayList[sl] = true
		}
	}
	for sl := range newL.ShowPlayList {
		if newL.ShowPlayList[sl] != l.ShowPlayList[sl] {
			changed = true
			playListChanged = true
			break
		}
	}
	newL.PlayList = make(map[string]map[string]Song)
	for sl := range l.ShowPlayList {
		newL.PlayList[sl] = make(map[string]Song)
		_, ok := l.PlayList[sl]
		for _, s := range l.playListSongsPerSlot {
			v := Song{}
			if ok {
				v = l.PlayList[sl][s]
			}
			fvf := r.FormValue(`play_list_file_` + sl + `_` + s)
			fvn := r.FormValue(`play_list_name_` + sl + `_` + s)
			fva := r.FormValue(`play_list_author_` + sl + `_` + s)
			if v.File != fvf || v.Name != fvn || v.Author != fva {
				changed = true
				playListChanged = true
				if v.File != fvf {
					v.Icon = ``
				}
			}
			newL.PlayList[sl][s] = Song{
				File:   fvf,
				Name:   fvn,
				Author: fva,
				Icon:   v.Icon,
			}
		}
	}
	if len(r.Form[`browse_list`]) > 0 {
		newL.BrowseList = append([]string{}, r.Form[`browse_list`]...)
		if strings.Join(newL.BrowseList, ``) != strings.Join(l.BrowseList, ``) {
			changed = true
			browseListChanged = true
		}
	} else {
		newL.BrowseList = []string{}
	}
	newL.LabelContent = ``
	labelContent := r.FormValue(`label_content`)
	for _, option := range l.labelContentOptions {
		if labelContent == option && labelContent != l.LabelContent {
			newL.LabelContent = labelContent
			changed = true
		}
	}
	if newL.LabelContent == `` {
		newL.LabelContent = l.LabelContent
	}

	if changed {
		newL.artworkInit()
		err = newL.save()
		if err == nil {
			*l = newL.copy()
			if randomListChanged {
				jukebox.randomListChanged <- true
			}
			if playListChanged {
				if userInterface.wsCanWrite {
					userInterface.screenMessageChannel <- `init`
				}
			}
			if browseListChanged && cfg.Skin == `browser` {
				//
			}
			msgOK[`Configuration changed`] = nil
		} else {
			logger.queue <- fmt.Sprint(err)
			msgErr[`Error saving data please try again`] = nil
		}
	}

	return
}

// AJAX call.
func (l *Lists) webAdminData() ([]byte, bool) {
	type item struct {
		ID     string  `json:"id"`
		Name   string  `json:"name"`
		File   string  `json:"file"`
		Folder string  `json:"folder"`
		Parent *string `json:"parent"`
	}

	var (
		err       error
		data      []item
		emptyData []byte = []byte(`[]`)
	)

	i := 1
	dirs := make(map[string]int)
	err = filepath.Walk(l.rootDir, func(path string, info os.FileInfo, err error) error {
		defer func() {
			i++
		}()

		if err != nil {
			logger.queue <- fmt.Sprint(err)
			return filepath.SkipDir
		}

		path = strings.TrimPrefix(path, l.rootDir)
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

	if len(data) == 0 {
		return emptyData, true
	}

	j, err := json.Marshal(&data)
	if err != nil {
		return emptyData, false
	}

	return j, true
}

func (l *Lists) artworkInit() (changed bool) {
	if _, err := os.Stat(l.artworkDir); os.IsNotExist(err) {
		if err = os.MkdirAll(l.artworkDir, 0755); err != nil {
			logger.queue <- fmt.Sprint(err)
			return
		}
	}
	for slot := range l.ShowPlayList {
		for key, song := range l.PlayList[slot] {
			if l.artwork(slot, key, &song) {
				l.PlayList[slot][key] = song
				changed = true
			}
		}
	}

	return
}

func (l *Lists) artwork(slot, key string, song *Song) (changed bool) {
	fileName := l.artworkDir + slot + `_` + key
	thumbFileName := l.artworkDir + slot + `_` + key + `_thumb`

	if song.Icon != `` {
		_, err := os.Stat(fileName + `.` + song.Icon)
		if !os.IsNotExist(err) {
			_, err := os.Stat(thumbFileName + `.` + song.Icon)
			if !os.IsNotExist(err) {
				return
			}
		}
	}

	song.Icon = ``
	changed = true

	// Remove previous artwork if exists.
	files, err := filepath.Glob(fileName + `*`)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	} else {
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				logger.queue <- fmt.Sprint(err)
			}
		}
	}

	if song.File == `` {
		return
	}

	file, err := os.Open(l.rootDir + song.File)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return
	}
	defer file.Close()

	meta, err := tag.ReadFrom(file)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return
	}
	pic := meta.Picture()
	if pic == nil || len(pic.Data) == 0 || pic.Ext == `` {
		return
	}
	img, err := os.Create(fileName + `.` + pic.Ext)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return
	}
	defer img.Close()
	_, err = img.Write(pic.Data)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return
	}
	img.Close()
	thumb, err := imaging.Open(fileName + `.` + pic.Ext)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
		return
	}
	thumb = imaging.Resize(thumb, 96, 0, imaging.NearestNeighbor)
	err = imaging.Save(thumb, thumbFileName+`.`+pic.Ext)
	if err != nil {
		logger.queue <- fmt.Sprint(err)
	} else {
		song.Icon = pic.Ext
	}

	return
}

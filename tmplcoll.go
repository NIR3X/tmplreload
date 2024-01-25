package tmplreload

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"
)

// A collection template.
type CollTmpl interface {
	Delims(left, right string)
	FuncAdd(name string, function interface{})
	FuncsAdd(funcMap template.FuncMap)
	FuncsRemove(names ...string)
	Option(opt ...string)
	Reload() error
	Execute(wr io.Writer, data interface{}) (err error)
}

// A function that determines whether or not a file should be parsed.
type DirFilter func(dir, path string, info os.FileInfo) bool

func DirFilterAll(dir, path string, info os.FileInfo) bool {
	return true
}

func DirFilterHTML(dir, path string, info os.FileInfo) bool {
	switch filepath.Ext(path) {
	case ".html", ".htm":
		return true
	default:
		return false
	}
}

// A struct that manages a collection of templates.
type TmplColl struct {
	mtx              sync.RWMutex
	modsMtx          sync.Mutex
	stopChan         chan struct{}
	wg               sync.WaitGroup
	cleanupIntvlSecs int64
	delims           [2]string
	funcMap          *funcMap
	options          map[string]string
	tmpls            map[string]*Tmpl
	dirs             map[string]DirFilter
}

// Creates a new TmplColl.
func New(cleanupIntvlSecs ...int64) *TmplColl {
	if len(cleanupIntvlSecs) == 0 {
		cleanupIntvlSecs = []int64{60}
	}

	tmplColl := &TmplColl{
		mtx:              sync.RWMutex{},
		modsMtx:          sync.Mutex{},
		stopChan:         make(chan struct{}),
		wg:               sync.WaitGroup{},
		cleanupIntvlSecs: cleanupIntvlSecs[0],
		delims:           [2]string{"{{", "}}"},
		funcMap:          newFuncMap(),
		options:          map[string]string{},
		tmpls:            map[string]*Tmpl{},
		dirs:             map[string]DirFilter{},
	}

	tmplColl.wg.Add(1)
	go func() {
		defer tmplColl.wg.Done()
		ticker := time.NewTicker(time.Duration(tmplColl.cleanupIntvlSecs) * time.Second)

		for {
			select {
			case <-ticker.C:
				tmplColl.IndexDirs()
				tmplColl.RemoveStaleTemplates()
			case <-tmplColl.stopChan:
				ticker.Stop()
				return
			}
		}
	}()

	return tmplColl
}

// Stops the TmplColl from removing stale templates and watching directories.
func (t *TmplColl) Stop() {
	close(t.stopChan)
	t.wg.Wait()
}

// Sets default delimiters for new templates.
func (t *TmplColl) Delims(left, right string) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.delims = [2]string{left, right}
}

// Adds the template function to the function map.
func (t *TmplColl) FuncAdd(name string, function interface{}) {
	t.funcMap.funcAdd(name, function)
	t.mtx.Lock()
	defer t.mtx.Unlock()
	for _, tmpl := range t.tmpls {
		tmpl.FuncAdd(name, function)
	}
}

// Adds the template functions to the function map.
func (t *TmplColl) FuncsAdd(funcMap template.FuncMap) {
	t.funcMap.funcsAdd(funcMap)
	t.mtx.Lock()
	defer t.mtx.Unlock()
	for _, tmpl := range t.tmpls {
		tmpl.FuncsAdd(funcMap)
	}
}

// Removes the template functions from the function map.
func (t *TmplColl) FuncsRemove(names ...string) {
	t.funcMap.funcsRemove(names...)
	t.mtx.Lock()
	defer t.mtx.Unlock()
	for _, tmpl := range t.tmpls {
		tmpl.FuncsRemove(names...)
	}
}

// Sets options for new templates.
func (t *TmplColl) Option(opt ...string) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	for _, o := range opt {
		index := strings.Index(o, "=")
		if index == -1 {
			continue
		}
		key := o[:index]
		value := o[index+1:]
		t.options[key] = value
	}
}

// Returns the template associated with the given filename.
func (t *TmplColl) Lookup(filename string) CollTmpl {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil
	}
	// This is required because the function returns the CollTmpl interface
	// and not the *Tmpl struct. If we were to return t.tmpls[absPath] directly,
	// it would cause a problem because we wouldn't be able to check if the template is nil.
	tmpl := t.tmpls[absPath]
	if tmpl == nil {
		return nil
	}
	return tmpl
}

// Executes the template associated with the given filename.
func (t *TmplColl) ExecuteTemplate(wr io.Writer, filename string, data interface{}) error {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	absPath, err := filepath.Abs(filename)
	if err != nil {
		return err
	}
	switch {
	case t.tmpls[absPath] == nil:
		return os.ErrNotExist
	default:
		return t.tmpls[absPath].Execute(wr, data)
	}
}

func (t *TmplColl) parseFiles(lockMods bool, filenames ...string) error {
	if lockMods {
		t.modsMtx.Lock()
		defer t.modsMtx.Unlock()
	}

	for _, filename := range filenames {
		absPath, err := filepath.Abs(filename)
		if err != nil {
			return err
		}

		tmpl := NewTmpl()
		tmpl.Delims(t.delims[0], t.delims[1])
		t.funcMap.accessFunctions(func(funcMap template.FuncMap) {
			tmpl.FuncsAdd(funcMap)
		})
		for key, value := range t.options {
			tmpl.Option(key + "=" + value)
		}
		if tmpl.ParseFile(absPath); err != nil {
			return err
		}

		t.mtx.Lock()
		t.tmpls[absPath] = tmpl
		t.mtx.Unlock()
	}

	return nil
}

// Parses the files and associates the resulting templates with filenames.
func (t *TmplColl) ParseFiles(filenames ...string) error {
	return t.parseFiles(true, filenames...)
}

// Parses the files and associates the resulting templates with filenames.
func (t *TmplColl) ParseGlob(pattern string) error {
	t.modsMtx.Lock()
	defer t.modsMtx.Unlock()

	filenames, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	return t.parseFiles(false, filenames...)
}

// Reloads the templates associated with the given filenames.
func (t *TmplColl) ReloadFiles(filenames ...string) error {
	t.modsMtx.Lock()
	defer t.modsMtx.Unlock()

	for _, filename := range filenames {
		absPath, err := filepath.Abs(filename)
		if err != nil {
			continue
		}
		t.mtx.RLock()
		tmpl := t.tmpls[absPath]
		t.mtx.RUnlock()
		if tmpl != nil {
			_ = tmpl.Reload()
		}
	}

	return nil
}

func (t *TmplColl) removeFiles(lockMods bool, filenames ...string) {
	if lockMods {
		t.modsMtx.Lock()
		defer t.modsMtx.Unlock()
	}

	for _, filename := range filenames {
		absPath, err := filepath.Abs(filename)
		if err != nil {
			continue
		}
		t.mtx.Lock()
		delete(t.tmpls, absPath)
		t.mtx.Unlock()
	}
}

// Removes the templates associated with the given filenames.
func (t *TmplColl) RemoveFiles(filenames ...string) {
	t.removeFiles(true, filenames...)
}

// Removes templates that no longer exist.
func (t *TmplColl) RemoveStaleTemplates() {
	t.modsMtx.Lock()
	defer t.modsMtx.Unlock()

	t.mtx.RLock()
	absPaths := make([]string, 0, len(t.tmpls))
	for absPath := range t.tmpls {
		absPaths = append(absPaths, absPath)
	}
	t.mtx.RUnlock()

	for _, absPath := range absPaths {
		_, err := os.Stat(absPath)
		if os.IsNotExist(err) {
			t.mtx.Lock()
			delete(t.tmpls, absPath)
			t.mtx.Unlock()
		}
	}
}

// Indexes the directories that are being watched immediately.
func (t *TmplColl) IndexDirs() {
	t.modsMtx.Lock()
	defer t.modsMtx.Unlock()

	t.mtx.RLock()
	dirs := make(map[string]DirFilter, len(t.dirs))
	for dir, dirFilter := range t.dirs {
		dirs[dir] = dirFilter
	}
	t.mtx.RUnlock()

	for dir, dirFilter := range dirs {
		if dirFilter == nil {
			dirFilter = DirFilterHTML
		}
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !dirFilter(dir, path, info) {
				return nil
			}
			t.parseFiles(false, path)
			return nil
		})
	}
}

// Adds the directory to the list of directories to watch for changes.
func (t *TmplColl) WatchDir(dir string, dirFilter DirFilter) error {
	t.modsMtx.Lock()
	defer t.modsMtx.Unlock()

	absPath, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	t.mtx.Lock()
	t.dirs[absPath] = dirFilter
	t.mtx.Unlock()

	return nil
}

// Removes the directory from the list of directories to watch for changes
// and removes the templates associated with the directory.
func (t *TmplColl) UnwatchDir(dir string) error {
	t.modsMtx.Lock()
	defer t.modsMtx.Unlock()

	absPath, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	t.mtx.Lock()
	delete(t.dirs, absPath)
	t.mtx.Unlock()

	filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		t.removeFiles(false, path)
		return nil
	})

	return nil
}

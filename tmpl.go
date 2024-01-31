package tmplreload

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"
)

// A template that reloads itself when the underlying file changes.
type Tmpl struct {
	mtx                sync.RWMutex
	minUpdateIntvlSecs int64
	delims             [2]string
	funcMap            *funcMap
	funcMapUpdated     bool
	options            map[string]string
	tmpl               *template.Template
	lastMod            int64
	lastUpdate         int64
	lastParsed         string
}

// Creates a new template.
func NewTmpl(minUpdateIntvlSecs ...int64) *Tmpl {
	if len(minUpdateIntvlSecs) == 0 {
		minUpdateIntvlSecs = []int64{1}
	}
	return &Tmpl{
		mtx:                sync.RWMutex{},
		minUpdateIntvlSecs: minUpdateIntvlSecs[0],
		delims:             [2]string{"{{", "}}"},
		funcMap:            newFuncMap(),
		funcMapUpdated:     false,
		options:            map[string]string{},
		tmpl:               nil,
		lastMod:            math.MinInt64,
		lastUpdate:         math.MinInt64,
	}
}

// Sets the delimiters to the specified strings.
func (t *Tmpl) Delims(left, right string) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	t.delims = [2]string{left, right}
}

// Adds the template function to the function map.
func (t *Tmpl) FuncAdd(name string, function interface{}) {
	t.mtx.Lock()
	t.funcMap.funcAdd(name, function)
	t.funcMapUpdated = true
	t.mtx.Unlock()
}

// Adds the template functions to the function map.
func (t *Tmpl) FuncsAdd(funcMap template.FuncMap) {
	t.mtx.Lock()
	t.funcMap.funcsAdd(funcMap)
	t.funcMapUpdated = true
	t.mtx.Unlock()
}

// Removes the template functions from the function map.
func (t *Tmpl) FuncsRemove(names ...string) {
	t.mtx.Lock()
	t.funcMap.funcsRemove(names...)
	t.funcMapUpdated = true
	t.mtx.Unlock()
}

// Sets options for the template.
func (t *Tmpl) Option(opt ...string) {
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
		if t.tmpl != nil {
			t.tmpl.Option(o)
		}
	}
}

func (t *Tmpl) parseFile(filename string, lock bool) (err error) {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return err
	}
	fileInfoModTime := fileInfo.ModTime().Unix()

	if lock {
		t.mtx.RLock()
	}
	modified := fileInfoModTime != t.lastMod || t.funcMapUpdated
	if lock {
		t.mtx.RUnlock()
	}

	if modified {
		if lock {
			t.mtx.Lock()
		}
		if fileInfoModTime != t.lastMod || t.funcMapUpdated {
			tmpl := template.New(filepath.Base(filename))
			tmpl.Delims(t.delims[0], t.delims[1])
			t.funcMap.accessFunctions(func(funcMap template.FuncMap) {
				tmpl.Funcs(funcMap)
			})
			for key, value := range t.options {
				tmpl.Option(key + "=" + value)
			}
			tmpl, err = tmpl.ParseFiles(filename)
			if err == nil {
				t.tmpl = tmpl
				t.lastMod = fileInfoModTime
				t.lastUpdate = time.Now().Unix()
				t.lastParsed = filename
			}
		}
		if lock {
			t.mtx.Unlock()
		}
	}

	return
}

// Parses the named file.
func (t *Tmpl) ParseFile(filename string) error {
	return t.parseFile(filename, true)
}

// Reloads the template immediately.
func (t *Tmpl) Reload() error {
	t.mtx.RLock()
	lastParsed := t.lastParsed
	t.mtx.RUnlock()
	return t.ParseFile(lastParsed)
}

// Executes the template.
func (t *Tmpl) Execute(wr io.Writer, data interface{}) (err error) {
	currentTime := time.Now().Unix()

	t.mtx.RLock()
	initiated := t.tmpl != nil
	updateRequired := t.minUpdateIntvlSecs != -1 && t.lastUpdate+t.minUpdateIntvlSecs <= currentTime
	t.mtx.RUnlock()

	if !initiated {
		return os.ErrNotExist
	}

	if updateRequired {
		t.mtx.Lock()
		if t.lastUpdate < currentTime {
			err = t.parseFile(t.lastParsed, false)
			t.lastUpdate = time.Now().Unix()
		}
		t.mtx.Unlock()
	}

	if err == nil {
		t.mtx.RLock()
		if t.tmpl == nil {
			err = os.ErrNotExist
		} else {
			err = t.tmpl.Execute(wr, data)
		}
		t.mtx.RUnlock()
	}

	return
}

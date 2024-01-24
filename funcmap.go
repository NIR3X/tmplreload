package tmplreload

import (
	"sync"
	"text/template"
)

type funcMap struct {
	mtx       sync.Mutex
	functions template.FuncMap
}

func newFuncMap() *funcMap {
	return &funcMap{
		mtx:       sync.Mutex{},
		functions: template.FuncMap{},
	}
}

func (f *funcMap) accessFunctions(callback func(template.FuncMap)) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	callback(f.functions)
}

func (f *funcMap) funcAdd(name string, function interface{}) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	f.functions[name] = function
}

func (f *funcMap) funcsAdd(funcMap template.FuncMap) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	for k, v := range funcMap {
		f.functions[k] = v
	}
}

func (f *funcMap) funcsRemove(names ...string) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	for _, name := range names {
		delete(f.functions, name)
	}
}

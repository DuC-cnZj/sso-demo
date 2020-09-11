package filters

import (
	"errors"
	"github.com/jinzhu/gorm"
)

var FieldNotFound = errors.New("field not found")

type GormScopeFunc = func(*gorm.DB) *gorm.DB

type Filterable interface {
	Apply() []GormScopeFunc
	GetFuncByName(string) func(Filterable) GormScopeFunc
	All() []string
	Push(GormScopeFunc)
	Scopes() []GormScopeFunc
	ResetScopes()
	SetInput(input map[string]interface{})
	Get(string) (interface{}, error)
	RegisterFilterFunc(string, func(Filterable) GormScopeFunc)
}

type Filter struct {
	input     map[string]interface{}
	scopes    []GormScopeFunc
	filters   map[string]func(Filterable) GormScopeFunc
	ApplyFunc func(f Filterable) []GormScopeFunc
}

func NewFilter(input map[string]interface{}) *Filter {
	return &Filter{input: input, scopes: make([]GormScopeFunc, 0), ApplyFunc: DefaultApply(), filters: map[string]func(Filterable) GormScopeFunc{}}
}

func (f *Filter) RegisterFilterFunc(name string, fn func(Filterable) GormScopeFunc) {
	f.filters[name] = fn
}

func (f *Filter) Get(s string) (interface{}, error) {
	if data, ok := f.input[s]; ok {
		return data, nil
	}
	return nil, FieldNotFound
}

func (f *Filter) SetInput(input map[string]interface{}) {
	f.input = input
}

func (f *Filter) All() []string {
	var all []string
	for s := range f.filters {
		all = append(all, s)
	}
	return all
}

func (f *Filter) Scopes() []GormScopeFunc {
	return f.scopes
}

func (f *Filter) ResetScopes() {
	f.scopes = make([]GormScopeFunc, 0)
}

func (f *Filter) GetFuncByName(s string) func(Filterable) GormScopeFunc {
	if fn, ok := f.filters[s]; ok {
		return fn
	}

	return nil
}

func (f *Filter) Apply() []GormScopeFunc {
	f.scopes = f.ApplyFunc(f)
	return f.scopes
}

func (f *Filter) Push(scope GormScopeFunc) {
	f.scopes = append(f.scopes, scope)
}

func DefaultApply() func(f Filterable) []GormScopeFunc {
	return func(f Filterable) []GormScopeFunc {
		f.ResetScopes()
		for _, key := range f.All() {
			if fn := f.GetFuncByName(key); fn != nil {
				f.Push(fn(f))
			}
		}

		return f.Scopes()
	}
}

package recorder

import (
	"fmt"
	"sync"
)

type Recorder[T Record] interface {
	Record(r T)
	Get() Records
	Print()
}

type recorder[T Record] struct {
	m        sync.RWMutex
	initOnce sync.Once
	records  records[T]
}

func New[T Record]() Recorder[T] {
	return &recorder[T]{}
}

func (r *recorder[T]) init() {
	r.initOnce.Do(func() {
		if r.records == nil {
			r.records = []T{}
		}
	})
}

func (r *recorder[T]) Record(rec T) {
	r.init()
	r.m.Lock()
	defer r.m.Unlock()
	r.records = append(r.records, rec)
}

func (r *recorder[T]) Get() Records {
	r.m.RLock()
	defer r.m.RUnlock()
	return r.records
}

func (r *recorder[T]) Print() {
	r.m.RLock()
	defer r.m.RUnlock()
	for _, d := range r.records {
		fmt.Println(d.GetDetails())
	}
}

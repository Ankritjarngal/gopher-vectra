package storage

import(
	"sync"
	"github.com/Ankritjarngal/gopher-vectra/pkg/vector"
)


type Memtable struct {
	mu sync.RWMutex
	entries map[string]*vector.Vector
	capacity int
}

func NewMemtable(cap int) *Memtable {
	return &Memtable{
		entries: make(map[string]*vector.Vector),
		capacity: cap,
	}
}

func(m * Memtable) Put(v *vector.Vector) bool{
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[v.ID] = v
	return len(m.entries) >= m.capacity
	
}

func (m * Memtable) Get(id string)(*vector.Vector,bool){
	m.mu.RLock()
	defer m.mu.RUnlock()
	v,ok:=m.entries[id]
	return v,ok
}

func (m * Memtable) Clear(){
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]*vector.Vector)

}

func (m * Memtable) GetEntries() map[string]*vector.Vector{
	m.mu.RLock()
	defer m.mu.RUnlock()
	copy:=make(map[string]*vector.Vector)
	for k,v:=range m.entries{
		copy[k]=v
	}
	return copy
}
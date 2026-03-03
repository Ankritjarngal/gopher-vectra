package bloom

import (
	"hash/fnv"
	"math"
)

type Filter struct {
	Bits []bool
	K    uint 
	M    uint 
}

func New(n int, p float64) *Filter {
	m := uint(-float64(n) * math.Log(p) / math.Pow(math.Log(2), 2))
	k := uint(float64(m) / float64(n) * math.Log(2))
	return &Filter{
		Bits: make([]bool, m),
		K:    k,
		M:    m,
	}
}

func (f *Filter) Add(id string) {
	h1, h2 := hash(id)
	for i := uint(0); i < f.K; i++ {
		idx := (uint(h1) + i*uint(h2)) % f.M
		f.Bits[idx] = true
	}
}

func (f *Filter) MightContain(id string) bool {
	h1, h2 := hash(id)
	for i := uint(0); i < f.K; i++ {
		idx := (uint(h1) + i*uint(h2)) % f.M
		if !f.Bits[idx] {
			return false
		}
	}
	return true
}

func hash(s string) (uint32, uint32) {
	h := fnv.New64a()
	h.Write([]byte(s))
	sum := h.Sum64()
	return uint32(sum), uint32(sum >> 32)
}
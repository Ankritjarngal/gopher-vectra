package vector


import "errors"

type Vector struct {
	ID       string            `json:"id"`
	Values   []float32         `json:"values"`
	Metadata map[string]string `json:"metadata"`
	Score    float64           `json:"-"` 
}


var(
	ErrDimensionMismatch = errors.New("vector dimensions do not match")
	ErrEmptyVector       = errors.New("vector is empty")
)
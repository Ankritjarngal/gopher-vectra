package vector


import "errors"

type Vector struct {
	ID string 
	Values []float32
	Metadata map[string]string
}


var(
	ErrDimensionMismatch = errors.New("vector dimensions do not match")
	ErrEmptyVector       = errors.New("vector is empty")
)
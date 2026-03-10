package vector

import (
	"gonum.org/v1/gonum/blas/blas32"
)


func Normalize(values []float32) {
	if len(values) == 0 {
		return
	}

	vec := blas32.Vector{Inc: 1, Data: values}

	magnitude := blas32.Nrm2(vec)

	if magnitude > 0 {
		blas32.Scal(1.0/magnitude, vec)
	}
}

func CosineSimilarity(v1, v2 []float32) (float64, error) {
	if len(v1) != len(v2) {
		return 0, ErrDimensionMismatch
	}

	vec1 := blas32.Vector{Inc: 1, Data: v1}
	vec2 := blas32.Vector{Inc: 1, Data: v2}
	dot := blas32.Dot(vec1, vec2)
	return float64(dot), nil
}
package vector

import (
	// "errors"
	"math"

	"gonum.org/v1/gonum/blas/blas32"
)

// var ErrDimensionMismatch = errors.New("dimension mismatch")

// EuclideanDistance calculates the L2 distance between two vectors.
// It uses BLAS Axpy (y = ax + y) to find the difference and Nrm2 for the norm.
func EuclideanDistance(v1, v2 []float32) (float64, error) {
	if len(v1) != len(v2) {
		return 0, ErrDimensionMismatch
	}

	// Create a temporary slice to hold the difference (v1 - v2)
	// We make a copy of v1 so we don't mutate the original input
	diff := make([]float32, len(v1))
	copy(diff, v1)

	// Wrap in blas32.Vector headers
	vecDiff := blas32.Vector{Inc: 1, Data: diff}
	vecV2 := blas32.Vector{Inc: 1, Data: v2}

	// Calculate: diff = diff + (-1 * v2) => v1 - v2
	blas32.Axpy(-1, vecV2, vecDiff)

	// Result is the L2 Norm (Euclidean Norm) of the difference vector
	return float64(blas32.Nrm2(vecDiff)), nil
}

// CosineSimilarity calculates the cosine of the angle between two vectors.
// Formula: (A · B) / (||A|| * ||B||)
func CosineSimilarity(v1, v2 []float32) (float64, error) {
	if len(v1) != len(v2) {
		return 0, ErrDimensionMismatch
	}

	vec1 := blas32.Vector{Inc: 1, Data: v1}
	vec2 := blas32.Vector{Inc: 1, Data: v2}

	// 1. Calculate Dot Product (A · B)
	dot := blas32.Dot(vec1, vec2)

	// 2. Calculate Magnitudes (L2 Norms)
	normV1 := blas32.Nrm2(vec1)
	normV2 := blas32.Nrm2(vec2)

	if normV1 == 0 || normV2 == 0 {
		return 0, nil
	}

	return float64(dot / (normV1 * normV2)), nil
}

// Normalize modifies the slice in-place to have a unit length of 1.
func (v *Vector) Normalize() {
	var sum float64
	for _, val := range v.Values {
		sum += float64(val * val)
	}
	magnitude := math.Sqrt(sum)
	if magnitude > 0 {
		for i := range v.Values {
			v.Values[i] = float32(float64(v.Values[i]) / magnitude)
		}
	}
}
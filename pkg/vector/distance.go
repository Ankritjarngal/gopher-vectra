package vector

import (
	"math"
)

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

func CosineSimilarity(v1, v2 []float32) (float64, error) {
	if len(v1) != len(v2) {
		return 0, ErrDimensionMismatch
	}

	var dotProduct, normV1, normV2 float64
	for i := range v1 {
		dotProduct += float64(v1[i] * v2[i])
		normV1 += float64(v1[i] * v1[i])
		normV2 += float64(v2[i] * v2[i])
	}

	if normV1 == 0 || normV2 == 0 {
		return 0, nil
	}

	return dotProduct / (math.Sqrt(normV1) * math.Sqrt(normV2)), nil
}
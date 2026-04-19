package vector

import (
	"math"
	"fmt"
)

func (v *Vector) Normalize() {
    var sum float32
    for _, val := range v.Values {
        sum += val * val
    }
    mag := float32(math.Sqrt(float64(sum)))
    if mag > 0 {
        for i := range v.Values {
            v.Values[i] /= mag
        }
    }
}

func CosineSimilarity(a, b []float32) (float64, error) {
    if len(a) != len(b) {
        return 0, fmt.Errorf("dimension mismatch")
    }
    var dotProduct, normA, normB float64
    for i := 0; i < len(a); i++ {
        dotProduct += float64(a[i]) * float64(b[i])
        normA += float64(a[i]) * float64(a[i])
        normB += float64(b[i]) * float64(b[i])
    }
    if normA == 0 || normB == 0 {
        return 0, nil
    }
    return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)), nil
}
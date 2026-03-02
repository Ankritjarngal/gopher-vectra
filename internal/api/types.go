package api

// import "github.com/Ankritjarngal/gopher-vectra/pkg/vector"

type UpsertRequest struct {
	ID     string    `json:"id"`
	Values []float32 `json:"values"`
}

type SearchRequest struct {
	Values []float32 `json:"values"`
	K      int       `json:"k"`
}

type SearchResponse struct {
	ID         string  `json:"id"`
	Similarity float64 `json:"similarity"`
}
package api

import (
	"encoding/json"
	"net/http"

	"github.com/Ankritjarngal/gopher-vectra/internal/engine"
	"github.com/Ankritjarngal/gopher-vectra/internal/storage"
	"github.com/Ankritjarngal/gopher-vectra/pkg/vector"
)

type Server struct {
	Index *engine.HNSW
	WAL   *storage.WAL
}

func (s *Server) HandleUpsert(w http.ResponseWriter, r *http.Request) {
	var req UpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	v := &vector.Vector{ID: req.ID, Values: req.Values}
	
	s.WAL.Write(v)
	
	s.Index.Insert(v)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (s *Server) HandleSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	results := s.Index.Search(req.Values, req.K)
	
	var response []SearchResponse
	for _, res := range results {
		sim, _ := vector.CosineSimilarity(req.Values, res.Values)
		response = append(response, SearchResponse{
			ID:         res.ID,
			Similarity: sim,
		})
	}

	json.NewEncoder(w).Encode(response)
}
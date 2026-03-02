package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ankritjarngal/gopher-vectra/internal/engine"
	"github.com/Ankritjarngal/gopher-vectra/internal/storage"
	"github.com/Ankritjarngal/gopher-vectra/pkg/vector"
)

var (
	index    *engine.HNSW
	wal      *storage.WAL
	memtable *storage.Memtable
)

func main() {
	var err error
	wal, err = storage.NewWAL("gopher.wal")
	if err != nil {
		log.Fatalf("Failed to open WAL: %v", err)
	}

	index = engine.NewHNSW(16, 50)
	memtable = storage.NewMemtable(5)

	recovered, _ := wal.ReadALL()

	if _, err := os.Stat("gopher.index"); err == nil {
		for _, v := range recovered {
			id := uint32(len(index.Nodes))
			index.Nodes[id] = &engine.Node{ID: id, Vector: v}
			memtable.Put(v)
		}
		if err := index.Load("gopher.index"); err != nil {
			rebuildIndex(recovered)
		}
	} else {
		rebuildIndex(recovered)
	}

	http.HandleFunc("/upsert", handleUpsert)
	http.HandleFunc("/search", handleSearch)
	http.HandleFunc("/status", handleStatus)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		index.Save("gopher.index")
		wal.Close()
		os.Exit(0)
	}()

	port := ":8080"
	fmt.Printf("Listening on %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func rebuildIndex(vectors []*vector.Vector) {
	index.Nodes = make(map[uint32]*engine.Node)
	index.CurrentMax = -1
	memtable.Clear()
	for _, v := range vectors {
		index.Insert(v)
		memtable.Put(v)
	}
}

func handleUpsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var v vector.Vector
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	wal.Write(&v)
	index.Insert(&v)

	if memtable.Put(&v) {
		data := memtable.GetEntries()
		filename := fmt.Sprintf("level0_%d.db", time.Now().UnixNano())
		storage.Flush(data, filename)
		memtable.Clear()
		fmt.Println("Flushed memtable to", filename)
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"status": "success", "id": "%s"}`, v.ID)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type searchReq struct {
		Values []float32 `json:"values"`
		K      int       `json:"k"`
	}

	var req searchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	results := index.Search(req.Values, req.K)

	type resultDTO struct {
		ID       string            `json:"id"`
		Score    float64           `json:"score"`
		Metadata map[string]string `json:"metadata"`
	}

	var output []resultDTO
	for _, res := range results {
		sim, _ := vector.CosineSimilarity(req.Values, res.Values)
		output = append(output, resultDTO{
			ID:       res.ID,
			Score:    sim,
			Metadata: res.Metadata,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(output)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"vectors_count": len(index.Nodes),
		"max_layer":     index.CurrentMax,
		"entry_node":    index.EntryNode,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
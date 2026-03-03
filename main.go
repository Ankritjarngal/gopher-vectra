package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/Ankritjarngal/gopher-vectra/internal/engine"
	"github.com/Ankritjarngal/gopher-vectra/internal/storage"
	"github.com/Ankritjarngal/gopher-vectra/pkg/vector"
)

var (
	index          *engine.HNSW
	wal            *storage.WAL
	memtable       *storage.Memtable
	startTime      time.Time
	insertionOrder []string
	maxRamLimit    = 500
)

func main() {
	startTime = time.Now()

	if dir := os.Getenv("VECTRA_DATA_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Cannot create data directory %s: %v", dir, err)
		}
		storage.DataDir = dir
		fmt.Printf("Data directory: %s\n", dir)
	}

	walPath := fmt.Sprintf("%s/gopher.wal", storage.DataDir)
	indexPath := fmt.Sprintf("%s/gopher.index", storage.DataDir)

	var err error
	wal, err = storage.NewWAL(walPath)
	if err != nil {
		log.Fatalf("Failed to open WAL: %v", err)
	}

	index = engine.NewHNSW(16, 50)
	memtable = storage.NewMemtable(50)

	if _, err := os.Stat(indexPath); err == nil {
		fmt.Println("Found index file, loading graph...")
		if err := index.Load(indexPath); err != nil {
			fmt.Printf("Index load failed (%v), falling back to WAL recovery\n", err)
			recovered, _ := wal.ReadALL()
			rebuildIndex(recovered)
		} else {
			fmt.Printf("Graph restored from index (%d nodes)\n", len(index.Nodes))
			if err := wal.Truncate(); err != nil {
				fmt.Printf("Warning: failed to truncate WAL: %v\n", err)
			} else {
				fmt.Println("WAL truncated")
			}

			ids := make([]uint32, 0, len(index.Nodes))
			for id := range index.Nodes {
				ids = append(ids, id)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
			for _, id := range ids {
				node := index.Nodes[id]
				if !node.Deleted {
					insertionOrder = append(insertionOrder, node.Vector.ID)
				}
			}
		}
	} else {
		fmt.Println("No index file found, replaying WAL...")
		recovered, _ := wal.ReadALL()
		rebuildIndex(recovered)
	}

	go func() {
		fmt.Println("Storage compactor started")
		for {
			time.Sleep(10 * time.Second)
			storage.RunCompaction()
		}
	}()

	http.HandleFunc("/upsert", handleUpsert)
	http.HandleFunc("/search", handleSearch)
	http.HandleFunc("/status", handleStatus)
	http.HandleFunc("/delete", handleDelete)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nSaving index and shutting down...")
		if err := index.Save(indexPath); err != nil {
			fmt.Printf("Warning: failed to save index: %v\n", err)
		}
		wal.Close()
		os.Exit(0)
	}()

	port := os.Getenv("VECTRA_PORT")
	if port == "" {
		port = ":8080"
	}
	fmt.Printf("GopherVectra listening on %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func rebuildIndex(vectors []*vector.Vector) {
	index.Nodes = make(map[uint32]*engine.Node)
	index.CurrentMax = -1
	memtable.Clear()
	insertionOrder = []string{}

	fmt.Printf("Recovering %d vectors...\n", len(vectors))
	for _, v := range vectors {
		index.Insert(v)
		memtable.Put(v)
		insertionOrder = append(insertionOrder, v.ID)
	}
	if memtable.Size() >= 50 {
		memtable.Clear()
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

	v.Normalize()

	if len(index.Nodes) >= maxRamLimit && len(insertionOrder) > 0 {
		victimID := insertionOrder[0]
		if internalID, found := index.FindInternalID(victimID); found {
			index.Delete(internalID)
			fmt.Printf("Evicted %s from RAM (memory limit reached)\n", victimID)
		}
		insertionOrder = insertionOrder[1:]
	}

	wal.Write(&v)
	index.Insert(&v)
	insertionOrder = append(insertionOrder, v.ID)

	if memtable.Put(&v) {
		data := memtable.GetEntries()
		filename := fmt.Sprintf("L0_%d.db", time.Now().UnixNano())
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

	tempVec := &vector.Vector{Values: req.Values}
	tempVec.Normalize()

	method := r.URL.Query().Get("method")

	type resultDTO struct {
		ID       string            `json:"id"`
		Score    float64           `json:"score"`
		Metadata map[string]string `json:"metadata"`
	}

	var output []resultDTO

	if method == "brute" {
		nodes := index.BruteForceSearch(tempVec.Values, req.K)
		for _, node := range nodes {
			sim, _ := vector.CosineSimilarity(tempVec.Values, node.Vector.Values)
			output = append(output, resultDTO{
				ID:       node.Vector.ID,
				Score:    sim,
				Metadata: node.Vector.Metadata,
			})
		}
	} else {
		ramResults := index.Search(tempVec.Values, req.K)
		diskResults := storage.SearchAllDiskLevels(tempVec.Values, req.K)

		uniqueResults := make(map[string]*vector.Vector)

		for _, res := range ramResults {
			if res == nil {
				continue
			}
			sim, _ := vector.CosineSimilarity(tempVec.Values, res.Values)
			res.Score = sim
			uniqueResults[res.ID] = res
		}

		for _, res := range diskResults {
			if res == nil {
				continue
			}
			if existing, found := uniqueResults[res.ID]; found {
				if res.Score > existing.Score {
					uniqueResults[res.ID] = res
				}
			} else {
				uniqueResults[res.ID] = res
			}
		}

		var finalResults []*vector.Vector
		for _, v := range uniqueResults {
			finalResults = append(finalResults, v)
		}

		sort.Slice(finalResults, func(i, j int) bool {
			return finalResults[i].Score > finalResults[j].Score
		})

		if len(finalResults) > req.K {
			finalResults = finalResults[:req.K]
		}

		for _, res := range finalResults {
			output = append(output, resultDTO{
				ID:       res.ID,
				Score:    res.Score,
				Metadata: res.Metadata,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(output)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	totalVectors := len(index.Nodes)
	stats := map[string]interface{}{
		"database_name": "GopherVectra",
		"uptime":        time.Since(startTime).String(),
		"storage": map[string]interface{}{
			"vectors_in_ram": totalVectors,
			"max_ram_limit":  maxRamLimit,
			"memtable_size":  memtable.Size(),
			"data_dir":       storage.DataDir,
		},
		"hnsw_metrics": map[string]interface{}{
			"max_layer":  index.CurrentMax,
			"entry_node": index.EntryNode,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Use DELETE", http.StatusMethodNotAllowed)
		return
	}
	idStr := r.URL.Query().Get("id")

	targetID, foundInRAM := index.FindInternalID(idStr)
	if foundInRAM {
		index.Delete(targetID)
		for i, id := range insertionOrder {
			if id == idStr {
				insertionOrder = append(insertionOrder[:i], insertionOrder[i+1:]...)
				break
			}
		}
	}

	tombstone := &vector.Vector{
		ID:       idStr,
		Metadata: map[string]string{"tombstone": "true"},
	}
	wal.Write(tombstone)
	memtable.Put(tombstone)

	foundOnDisk := storage.ExistOnDisk(idStr)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "deleted", "id": "%s", "from_ram": %v, "from_disk": %v}`, idStr, foundInRAM, foundOnDisk)
}
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
    "sort"

    "github.com/Ankritjarngal/gopher-vectra/internal/engine"
    "github.com/Ankritjarngal/gopher-vectra/internal/storage"
    "github.com/Ankritjarngal/gopher-vectra/pkg/vector"
)

var (
    index     *engine.HNSW
    wal       *storage.WAL
    memtable  *storage.Memtable
    startTime time.Time
)

func main() {
    startTime = time.Now()
    var err error
    
    wal, err = storage.NewWAL("gopher.wal")
    if err != nil {
        log.Fatalf("Failed to open WAL: %v", err)
    }

    index = engine.NewHNSW(16, 50)
    memtable = storage.NewMemtable(50)

    recovered, _ := wal.ReadALL()

    // Standard recovery process
    if _, err := os.Stat("gopher.index"); err == nil {
        if err := index.Load("gopher.index"); err != nil {
            rebuildIndex(recovered)
        } else {
            for _, v := range recovered {
                memtable.Put(v)
            }
            if memtable.Size() >= 50 {
                memtable.Clear()
            }
        }
    } else {
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
        fmt.Println("\nSaving index and shutting down")
        index.Save("gopher.index")
        wal.Close()
        os.Exit(0)
    }()

    port := os.Getenv("VECTRA_PORT")
    if port == "" {
        port = ":8080"
    }
    fmt.Printf("GopherVectra Listening on %s\n", port)
    log.Fatal(http.ListenAndServe(port, nil))
}

func rebuildIndex(vectors []*vector.Vector) {
    index.Nodes = make(map[uint32]*engine.Node)
    index.CurrentMax = -1
    memtable.Clear()
    
    fmt.Printf("Recovering %d vectors...\n", len(vectors))
    for _, v := range vectors {
        index.Insert(v)
        memtable.Put(v)
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

    tempVec := &vector.Vector{Values: req.Values}
    tempVec.Normalize()

    ramResults := index.Search(tempVec.Values, req.K)

    diskResults := storage.SearchAllDiskLevels(tempVec.Values, req.K)
    
    uniqueResults := make(map[string]*vector.Vector)

    for _, res := range ramResults {
        if res == nil { continue }
        sim, _ := vector.CosineSimilarity(tempVec.Values, res.Values)
        res.Score = sim
        uniqueResults[res.ID] = res
    }

    for _, res := range diskResults {
        if res == nil { continue }
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

    type resultDTO struct {
        ID       string            `json:"id"`
        Score    float64           `json:"score"`
        Metadata map[string]string `json:"metadata"`
    }

    var output []resultDTO
    for _, res := range finalResults {
        output = append(output, resultDTO{
            ID:       res.ID,
            Score:    res.Score,
            Metadata: res.Metadata,
        })
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
            "vectors_in_ram":    memtable.Size(),
            "total_vectors_idx": totalVectors,
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
    targetID, found := index.FindInternalID(idStr)
    if !found {
        http.Error(w, "Not found", http.StatusNotFound)
        return
    }
    index.Delete(targetID)
    w.Header().Set("Content-Type", "application/json")
    fmt.Fprintf(w, `{"status": "deleted", "id": "%s"}`, idStr)
}
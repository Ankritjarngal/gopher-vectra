package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"github.com/Ankritjarngal/gopher-vectra/pkg/vector"


)

func RunCompaction() {
	files, _ := filepath.Glob("level0_*.db")
	if len(files) < 10 {
		return
	}

	fmt.Printf("Compactor: Merging %d files...\n", len(files))
	mergedData := make(map[string]*vector.Vector)

	for _, f := range files {
		data, err := LoadSSTable(f)
		if err == nil {
			for id, v := range data {
				mergedData[id] = v 
			}
		}
	}

	newName := fmt.Sprintf("level1_%d.db", time.Now().Unix())
	_, err := Flush(mergedData, newName) 

	if err == nil {
		for _, f := range files {
			os.Remove(f)
		}
		fmt.Printf("Compaction successful! Created %s\n", newName)
	}
}
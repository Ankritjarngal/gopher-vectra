package storage

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func RunCompaction() {
	files, _ := os.ReadDir(".")
	
	l0Files := []string{}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "L0_") && strings.HasSuffix(f.Name(), ".db") {
			l0Files = append(l0Files, f.Name())
		}
	}

	if len(l0Files) >= 8 {
		compactLevel(l0Files, 0)
	}

	l1Files := []string{}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "L1_") && strings.HasSuffix(f.Name(), ".db") {
			l1Files = append(l1Files, f.Name())
		}
	}

	if len(l1Files) >= 15 {
		compactLevel(l1Files, 1)
	}
}

func compactLevel(filenames []string, currentLevel int) {
	mergedData, _ := LoadSSTable(filenames[0])
	
	for i := 1; i < len(filenames); i++ {
		data, err := LoadSSTable(filenames[i])
		if err == nil {
			for id, v := range data {
				mergedData[id] = v
			}
		}
	}

	for id, v := range mergedData {
		if v.Metadata != nil {
			if _, deleted := v.Metadata["tombstone"]; deleted {
				delete(mergedData, id)
			}
		}
	}

	nextLevel := currentLevel + 1
	newName := fmt.Sprintf("L%d_%d.db", nextLevel, time.Now().Unix())
	
	_, err := Flush(mergedData, newName)
	if err == nil {
		for _, f := range filenames {
			os.Remove(f)
			delete(ActiveFilters, f)
		}
		fmt.Printf("Level %d Compaction: Created %s\n", currentLevel, newName)
	}
}
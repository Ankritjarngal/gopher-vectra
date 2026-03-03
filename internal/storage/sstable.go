package storage

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Ankritjarngal/gopher-vectra/pkg/bloom"
	"github.com/Ankritjarngal/gopher-vectra/pkg/vector"
)

var ActiveFilters = make(map[string]*bloom.Filter)

type SSTable struct {
	Path string
}

func Flush(entries map[string]*vector.Vector, path string) (*SSTable, error) {
	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	bf := bloom.New(100, 0.01)

	for _, id := range ids {
		v := entries[id]
		bf.Add(id)

		idBytes := []byte(v.ID)
		binary.Write(f, binary.LittleEndian, uint32(len(idBytes)))
		f.Write(idBytes)

		isTombstone := uint8(0)
		if v.Metadata != nil {
			if _, deleted := v.Metadata["tombstone"]; deleted {
				isTombstone = 1
			}
		}
		binary.Write(f, binary.LittleEndian, isTombstone)

		binary.Write(f, binary.LittleEndian, uint32(len(v.Values)))
		binary.Write(f, binary.LittleEndian, v.Values)
	}
	ActiveFilters[filepath.Base(path)] = bf

	fmt.Printf("Successfully flushed %d vectors and created Bloom Filter for %s\n", len(entries), path)
	return &SSTable{Path: path}, nil
}

func LoadSSTable(path string) (map[string]*vector.Vector, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries := make(map[string]*vector.Vector)
	fileInfo, _ := f.Stat()
	size := fileInfo.Size()

	bf := bloom.New(100, 0.01)

	var offset int64 = 0
	for offset < size {
		var idLen uint32
		if err := binary.Read(f, binary.LittleEndian, &idLen); err != nil {
			break
		}
		idBytes := make([]byte, idLen)
		f.Read(idBytes)
		id := string(idBytes)

		var isTombstone uint8
		binary.Read(f, binary.LittleEndian, &isTombstone)

		var vecLen uint32
		binary.Read(f, binary.LittleEndian, &vecLen)
		values := make([]float32, vecLen)
		binary.Read(f, binary.LittleEndian, &values)

		v := &vector.Vector{ID: id, Values: values}
		if isTombstone == 1 {
			v.Metadata = map[string]string{"tombstone": "true"}
		}

		entries[id] = v
		bf.Add(id)
		offset, _ = f.Seek(0, 1)
	}

	ActiveFilters[filepath.Base(path)] = bf
	return entries, nil
}

func ExistOnDisk(id string) bool {
	files, _ := os.ReadDir(".")
	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), "L") && filepath.Ext(f.Name()) == ".db" {
			bf, exists := ActiveFilters[f.Name()]
			if exists && bf.MightContain(id) {
				entries, _ := LoadSSTable(f.Name())
				if vec, found := entries[id]; found {
					if vec.Metadata != nil {
						if _, deleted := vec.Metadata["tombstone"]; deleted {
							return false
						}
					}
					return true
				}
			}
		}
	}
	return false
}

func SearchSSTable(filename string, query []float32, k int) ([]*vector.Vector, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var results []*vector.Vector
	fileInfo, _ := f.Stat()
	size := fileInfo.Size()

	var offset int64 = 0
	for offset < size {
		var idLen uint32
		if err := binary.Read(f, binary.LittleEndian, &idLen); err != nil {
			break
		}
		idBytes := make([]byte, idLen)
		f.Read(idBytes)
		id := string(idBytes)

		var isTombstone uint8
		binary.Read(f, binary.LittleEndian, &isTombstone)

		var vecLen uint32
		binary.Read(f, binary.LittleEndian, &vecLen)
		values := make([]float32, vecLen)
		binary.Read(f, binary.LittleEndian, &values)

		if isTombstone == 1 {
			offset, _ = f.Seek(0, 1)
			continue
		}

		sim, _ := vector.CosineSimilarity(query, values)
		results = append(results, &vector.Vector{
			ID:     id,
			Values: values,
			Score:  sim,
		})
		offset, _ = f.Seek(0, 1)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > k {
		return results[:k], nil
	}
	return results, nil
}

func SearchAllDiskLevels(query []float32, k int) []*vector.Vector {
	files, _ := os.ReadDir(".")
	var allDiskResults []*vector.Vector
	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), "L") && filepath.Ext(f.Name()) == ".db" {
			results, err := SearchSSTable(f.Name(), query, k)
			if err == nil {
				allDiskResults = append(allDiskResults, results...)
			}
		}
	}
	sort.Slice(allDiskResults, func(i, j int) bool { return allDiskResults[i].Score > allDiskResults[j].Score })
	if len(allDiskResults) > k {
		return allDiskResults[:k]
	}
	return allDiskResults
}
package storage


import(
	"encoding/binary"
	"fmt"
	"os"
	"sort"

	"github.com/Ankritjarngal/gopher-vectra/pkg/vector"
)

type SSTable struct{
	Path string
}

func Flush(entries map[string]*vector.Vector,path string)(*SSTable,error){
	ids:=make([]string,0,len(entries))
	for id:=range entries{
		ids=append(ids,id)
	}
	sort.Strings(ids)
	f,err:=os.Create(path)
	if err!=nil{
		return nil,err
	}
	defer f.Close()
	for _, id := range ids {
		v := entries[id]		
		idBytes := []byte(v.ID)
		if err := binary.Write(f, binary.LittleEndian, uint32(len(idBytes))); err != nil {
			return nil, err
		}
		if _, err := f.Write(idBytes); err != nil {
			return nil, err
		}
		if err := binary.Write(f, binary.LittleEndian, uint32(len(v.Values))); err != nil {
			return nil, err
		}
		if err := binary.Write(f, binary.LittleEndian, v.Values); err != nil {
			return nil, err
		}
	}

	fmt.Printf("Successfully flushed %d vectors to %s\n", len(entries), path)
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

	var offset int64 = 0
	for offset < size {
		var idLen uint32
		if err := binary.Read(f, binary.LittleEndian, &idLen); err != nil {
			break 
		}

		idBytes := make([]byte, idLen)
		if _, err := f.Read(idBytes); err != nil {
			return nil, err
		}
		id := string(idBytes)

		var vecLen uint32
		if err := binary.Read(f, binary.LittleEndian, &vecLen); err != nil {
			return nil, err
		}

		values := make([]float32, vecLen)
		if err := binary.Read(f, binary.LittleEndian, &values); err != nil {
			return nil, err
		}

		entries[id] = &vector.Vector{
			ID:     id,
			Values: values,
		}
		offset, _ = f.Seek(0, 1)
	}

	return entries, nil
}

func SearchSSTable(filename string, query []float32, k int) ([]*vector.Vector, error) {
    f, err := os.Open(filename)
    if err != nil {
        return nil, err // File might not exist yet, that's okay
    }
    defer f.Close()

    var results []*vector.Vector
    fileInfo, _ := f.Stat()
    size := fileInfo.Size()

    var offset int64 = 0
    // Binary loop reading exactly like LoadSSTable
    for offset < size {
        var idLen uint32
        if err := binary.Read(f, binary.LittleEndian, &idLen); err != nil {
            break 
        }

        idBytes := make([]byte, idLen)
        if _, err := f.Read(idBytes); err != nil {
            break
        }
        id := string(idBytes)

        var vecLen uint32
        if err := binary.Read(f, binary.LittleEndian, &vecLen); err != nil {
            break
        }

        values := make([]float32, vecLen)
        if err := binary.Read(f, binary.LittleEndian, &values); err != nil {
            break
        }

        // Calculate similarity for this specific vector
        sim, _ := vector.CosineSimilarity(query, values)
        
        // Create the vector object with the score populated
        v := &vector.Vector{
            ID:     id,
            Values: values,
            Score:  sim,
        }
        results = append(results, v)
        
        offset, _ = f.Seek(0, 1) // Advance offset tracker
    }

    // Sort the results from this specific file (Highest score first)
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })

    // Trim to K if necessary
    if len(results) > k {
        return results[:k], nil
    }
    return results, nil
}

func SearchAllDiskLevels(query []float32, k int) []*vector.Vector {
	files, _ := os.ReadDir(".")
	var dbFiles []string
	for _, f := range files {
		if !f.IsDir() && (len(f.Name()) > 3 && f.Name()[len(f.Name())-3:] == ".db") {
			dbFiles = append(dbFiles, f.Name())
		}
	}

	var allDiskResults []*vector.Vector

	for _, filename := range dbFiles {
		results, err := SearchSSTable(filename, query, k)
		if err == nil {
			allDiskResults = append(allDiskResults, results...)
		}
	}

	sort.Slice(allDiskResults, func(i, j int) bool {
		return allDiskResults[i].Score > allDiskResults[j].Score
	})

	if len(allDiskResults) > k {
		return allDiskResults[:k]
	}
	return allDiskResults
}
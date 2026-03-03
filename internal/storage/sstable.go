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
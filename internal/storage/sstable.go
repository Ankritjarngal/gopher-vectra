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


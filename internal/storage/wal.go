package storage

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"sync"

	"github.com/Ankritjarngal/gopher-vectra/pkg/vector"
)

type WAL struct {
	file *os.File
	mu   sync.Mutex
}

func NewWAL(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{file: f}, nil
}

func (w *WAL) Write(v *vector.Vector) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	idBytes := []byte(v.ID)
	idLen := uint32(len(idBytes))
	dimLen := uint32(len(v.Values))

	metaBytes, _ := json.Marshal(v.Metadata)
	metaLen := uint32(len(metaBytes))

	binary.Write(w.file, binary.LittleEndian, idLen)
	w.file.Write(idBytes)
	binary.Write(w.file, binary.LittleEndian, dimLen)
	binary.Write(w.file, binary.LittleEndian, v.Values)
	
	binary.Write(w.file, binary.LittleEndian, metaLen)
	w.file.Write(metaBytes)

	return w.file.Sync()
}

func (w *WAL) Close() error {
	return w.file.Close()
}

func (w *WAL) ReadALL() ([]*vector.Vector, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Seek(0, 0); err != nil {
		return nil, err
	}

	var vectors []*vector.Vector
	for {
		var idLen uint32
		if err := binary.Read(w.file, binary.LittleEndian, &idLen); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		idBytes := make([]byte, idLen)
		io.ReadFull(w.file, idBytes)

		var dimLen uint32
		binary.Read(w.file, binary.LittleEndian, &dimLen)

		values := make([]float32, dimLen)
		binary.Read(w.file, binary.LittleEndian, &values)

		var metaLen uint32
		if err := binary.Read(w.file, binary.LittleEndian, &metaLen); err != nil {
			return nil, err
		}

		metaBytes := make([]byte, metaLen)
		io.ReadFull(w.file, metaBytes)

		var metadata map[string]string
		json.Unmarshal(metaBytes, &metadata)

		vectors = append(vectors, &vector.Vector{
			ID:       string(idBytes),
			Values:   values,
			Metadata: metadata,
		})
	}
	return vectors, nil
}
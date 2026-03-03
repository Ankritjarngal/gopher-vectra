package engine

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"sync"

	"github.com/Ankritjarngal/gopher-vectra/pkg/vector"
)

type Node struct {
	ID        uint32
	Vector    *vector.Vector
	Neighbors [][]uint32
	Deleted   bool
}

type HNSW struct {
	mu         sync.RWMutex
	MaxLayers  int
	M          int
	Ef         int
	ML         float64
	Nodes      map[uint32]*Node
	EntryNode  uint32
	CurrentMax int
}

func NewHNSW(m int, ef int) *HNSW {
	return &HNSW{
		Nodes:      make(map[uint32]*Node),
		M:          m,
		Ef:         ef,
		MaxLayers:  16,
		ML:         1.0 / math.Log(float64(m)),
		CurrentMax: -1,
	}
}

func (h *HNSW) RandomLevel() int {
	r := rand.Float64()
	if r == 0 {
		r = 1e-9
	}
	level := int(-math.Log(r) * h.ML)
	if level > h.MaxLayers {
		return h.MaxLayers
	}
	return level
}

func (h *HNSW) IsDeleted(id uint32) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if node, ok := h.Nodes[id]; ok {
		return node.Deleted
	}
	return true
}

func (h *HNSW) Insert(v *vector.Vector) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	nodeID := uint32(len(h.Nodes))
	targetLevel := h.RandomLevel()

	newNode := &Node{
		ID:        nodeID,
		Vector:    v,
		Neighbors: make([][]uint32, targetLevel+1),
		Deleted:   false,
	}

	for i := 0; i <= targetLevel; i++ {
		newNode.Neighbors[i] = make([]uint32, 0, h.M)
	}

	h.Nodes[nodeID] = newNode

	if h.CurrentMax == -1 {
		h.EntryNode = nodeID
		h.CurrentMax = targetLevel
		return nil
	}

	currObj := h.EntryNode
	for l := h.CurrentMax; l > targetLevel; l-- {
		res := h.internalSearchLayer(v.Values, currObj, 1, l)
		if len(res) > 0 {
			currObj = res[0]
		}
	}

	for l := minInt(targetLevel, h.CurrentMax); l >= 0; l-- {
		neighbors := h.internalSearchLayer(v.Values, currObj, h.M, l)

		for _, neighborID := range neighbors {
			newNode.Neighbors[l] = append(newNode.Neighbors[l], neighborID)
			if neighborNode, ok := h.Nodes[neighborID]; ok {
				neighborNode.Neighbors[l] = append(neighborNode.Neighbors[l], nodeID)
			}
		}
		if len(neighbors) > 0 {
			currObj = neighbors[0]
		}
	}

	if targetLevel > h.CurrentMax {
		h.EntryNode = nodeID
		h.CurrentMax = targetLevel
	}

	return nil
}

func (h *HNSW) internalSearchLayer(target []float32, entryNode uint32, ef int, level int) []uint32 {
	visited := map[uint32]bool{entryNode: true}
	candidates := []uint32{entryNode}
	foundNeighbors := []uint32{entryNode}

	for len(candidates) > 0 {
		currID := candidates[0]
		candidates = candidates[1:]

		currNode, ok := h.Nodes[currID]
		if !ok {
			continue
		}

		for _, neighborID := range currNode.Neighbors[level] {
			if !visited[neighborID] {
				visited[neighborID] = true
				if _, exists := h.Nodes[neighborID]; exists {
					foundNeighbors = append(foundNeighbors, neighborID)
					candidates = append(candidates, neighborID)
				}
			}
		}

		sort.Slice(foundNeighbors, func(i, j int) bool {
			nodeI := h.Nodes[foundNeighbors[i]]
			nodeJ := h.Nodes[foundNeighbors[j]]
			if nodeI == nil || nodeJ == nil {
				return false
			}
			simI, _ := vector.CosineSimilarity(target, nodeI.Vector.Values)
			simJ, _ := vector.CosineSimilarity(target, nodeJ.Vector.Values)
			return simI > simJ
		})

		if len(foundNeighbors) > ef {
			foundNeighbors = foundNeighbors[:ef]
		}
	}
	return foundNeighbors
}

func (h *HNSW) Search(query []float32, k int) []*vector.Vector {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.CurrentMax == -1 {
		return nil
	}

	currObj := h.EntryNode
	for l := h.CurrentMax; l > 0; l-- {
		res := h.internalSearchLayer(query, currObj, 1, l)
		if len(res) > 0 {
			currObj = res[0]
		}
	}

	resultIDs := h.internalSearchLayer(query, currObj, h.Ef, 0)
	results := make([]*vector.Vector, 0, len(resultIDs))
	for _, id := range resultIDs {
		node := h.Nodes[id]
		if node != nil && !node.Deleted {
			results = append(results, node.Vector)
		}
		if len(results) >= k {
			break
		}
	}
	return results
}

func (h *HNSW) Save(path string) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	binary.Write(f, binary.LittleEndian, uint32(h.EntryNode))
	binary.Write(f, binary.LittleEndian, int32(h.CurrentMax))
	binary.Write(f, binary.LittleEndian, uint32(len(h.Nodes)))

	for i := uint32(0); i < uint32(len(h.Nodes)); i++ {
		node := h.Nodes[i]
		layerCount := uint32(len(node.Neighbors))
		binary.Write(f, binary.LittleEndian, layerCount)
		binary.Write(f, binary.LittleEndian, node.Deleted)

		for l := 0; l < int(layerCount); l++ {
			neighborCount := uint32(len(node.Neighbors[l]))
			binary.Write(f, binary.LittleEndian, neighborCount)
			binary.Write(f, binary.LittleEndian, node.Neighbors[l])
		}
	}
	return nil
}

func (h *HNSW) Load(path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var entryNode uint32
	var currentMax int32
	var nodeCount uint32

	binary.Read(f, binary.LittleEndian, &entryNode)
	binary.Read(f, binary.LittleEndian, &currentMax)
	binary.Read(f, binary.LittleEndian, &nodeCount)

	h.EntryNode = entryNode
	h.CurrentMax = int(currentMax)

	for i := uint32(0); i < nodeCount; i++ {
		var layerCount uint32
		binary.Read(f, binary.LittleEndian, &layerCount)

		var isDeleted bool
		binary.Read(f, binary.LittleEndian, &isDeleted)

		node, exists := h.Nodes[i]
		if !exists {
			return fmt.Errorf("node %d not found in memory", i)
		}

		node.Deleted = isDeleted
		node.Neighbors = make([][]uint32, layerCount)
		for l := 0; l < int(layerCount); l++ {
			var neighborCount uint32
			binary.Read(f, binary.LittleEndian, &neighborCount)
			neighbors := make([]uint32, neighborCount)
			binary.Read(f, binary.LittleEndian, &neighbors)
			node.Neighbors[l] = neighbors
		}
	}
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *HNSW) Delete(id uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if node, ok := h.Nodes[id]; ok {
		node.Deleted = true
	}
}

func (h *HNSW) FindInternalID(stringID string) (uint32, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for id, node := range h.Nodes {
		if node.Vector.ID == stringID && !node.Deleted {
			return id, true
		}
	}
	return 0, false
}

func (h *HNSW) BruteForceSearch(query []float32, k int) []*Node {
	h.mu.RLock()
	defer h.mu.RUnlock()

	type distanceResult struct {
		node *Node
		dist float32
	}

	var allDistances []distanceResult

	for _, node := range h.Nodes {
		if node.Deleted {
			continue
		}
		sim, _ := vector.CosineSimilarity(query, node.Vector.Values)
		allDistances = append(allDistances, distanceResult{
			node: node,
			dist: float32(sim),
		})
	}

	sort.Slice(allDistances, func(i, j int) bool {
		return allDistances[i].dist > allDistances[j].dist
	})

	if len(allDistances) < k {
		k = len(allDistances)
	}

	results := make([]*Node, k)
	for i := 0; i < k; i++ {
		results[i] = allDistances[i].node
	}

	return results
}
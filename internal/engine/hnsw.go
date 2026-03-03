package engine

import (
	"container/heap"
	"encoding/binary"
	"fmt"
	"io"
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
	nextID     uint32
}

func NewHNSW(m int, ef int) *HNSW {
	return &HNSW{
		Nodes:      make(map[uint32]*Node),
		M:          m,
		Ef:         ef,
		MaxLayers:  16,
		ML:         1.0 / math.Log(float64(m)),
		CurrentMax: -1,
		nextID:     0,
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

	nodeID := h.nextID
	h.nextID++

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
			currObj = res[0].id
		}
	}

	for l := minInt(targetLevel, h.CurrentMax); l >= 0; l-- {
		neighbors := h.internalSearchLayer(v.Values, currObj, h.M, l)

		for _, item := range neighbors {
			newNode.Neighbors[l] = append(newNode.Neighbors[l], item.id)
			if neighborNode, ok := h.Nodes[item.id]; ok {
				neighborNode.Neighbors[l] = append(neighborNode.Neighbors[l], nodeID)
				if len(neighborNode.Neighbors[l]) > h.M {
					neighborNode.Neighbors[l] = h.pruneNeighbors(neighborNode.Vector.Values, neighborNode.Neighbors[l], h.M)
				}
			}
		}

		if len(newNode.Neighbors[l]) > h.M {
			newNode.Neighbors[l] = h.pruneNeighbors(v.Values, newNode.Neighbors[l], h.M)
		}

		if len(neighbors) > 0 {
			currObj = neighbors[0].id
		}
	}

	if targetLevel > h.CurrentMax {
		h.EntryNode = nodeID
		h.CurrentMax = targetLevel
	}

	return nil
}

func (h *HNSW) pruneNeighbors(base []float32, candidates []uint32, maxM int) []uint32 {
	type scored struct {
		id  uint32
		sim float64
	}

	var scoredCandidates []scored
	for _, id := range candidates {
		node, ok := h.Nodes[id]
		if !ok || node.Deleted {
			continue
		}
		sim, _ := vector.CosineSimilarity(base, node.Vector.Values)
		scoredCandidates = append(scoredCandidates, scored{id, sim})
	}

	sort.Slice(scoredCandidates, func(i, j int) bool {
		return scoredCandidates[i].sim > scoredCandidates[j].sim
	})

	selected := make([]uint32, 0, maxM)
	for _, c := range scoredCandidates {
		if len(selected) >= maxM {
			break
		}
		cNode, ok := h.Nodes[c.id]
		if !ok {
			continue
		}
		dominated := false
		for _, selectedID := range selected {
			sNode, ok := h.Nodes[selectedID]
			if !ok {
				continue
			}
			simToSelected, _ := vector.CosineSimilarity(cNode.Vector.Values, sNode.Vector.Values)
			if simToSelected > c.sim {
				dominated = true
				break
			}
		}
		if !dominated {
			selected = append(selected, c.id)
		}
	}

	if len(selected) < maxM {
		inSelected := make(map[uint32]bool, len(selected))
		for _, id := range selected {
			inSelected[id] = true
		}
		for _, c := range scoredCandidates {
			if len(selected) >= maxM {
				break
			}
			if !inSelected[c.id] {
				selected = append(selected, c.id)
			}
		}
	}

	return selected
}

func (h *HNSW) internalSearchLayer(target []float32, entryNode uint32, ef int, level int) []heapItem {
	entryNode_, ok := h.Nodes[entryNode]
	if !ok {
		return nil
	}
	entrySim, _ := vector.CosineSimilarity(target, entryNode_.Vector.Values)

	visited := map[uint32]bool{entryNode: true}

	// candidates: min-heap — always explore the closest unvisited node first
	candidates := &minHeap{{id: entryNode, sim: entrySim}}
	heap.Init(candidates)

	// results: max-heap — index 0 is always the worst result currently held
	// so we can evict it in O(log ef) when a better candidate arrives
	results := &worstHeap{{id: entryNode, sim: entrySim}}
	heap.Init(results)

	for candidates.Len() > 0 {
		best := heap.Pop(candidates).(heapItem)

		// the closest candidate we have not explored is worse than our worst
		// current result — no further exploration can improve results
		if results.Len() >= ef && best.sim < (*results)[0].sim {
			break
		}

		currNode, ok := h.Nodes[best.id]
		if !ok {
			continue
		}
		if level >= len(currNode.Neighbors) {
			continue
		}

		for _, neighborID := range currNode.Neighbors[level] {
			if visited[neighborID] {
				continue
			}
			visited[neighborID] = true

			neighborNode, ok := h.Nodes[neighborID]
			if !ok {
				continue
			}

			sim, _ := vector.CosineSimilarity(target, neighborNode.Vector.Values)

			// only add to results if we have room or this beats the worst result
			if results.Len() < ef || sim > (*results)[0].sim {
				heap.Push(candidates, heapItem{id: neighborID, sim: sim})
				heap.Push(results, heapItem{id: neighborID, sim: sim})
				if results.Len() > ef {
					heap.Pop(results) // evict worst
				}
			}
		}
	}

	out := make([]heapItem, results.Len())
	copy(out, *results)
	sort.Slice(out, func(i, j int) bool {
		return out[i].sim > out[j].sim
	})
	return out
}

type heapItem struct {
	id  uint32
	sim float64
}

type minHeap []heapItem

func (h minHeap) Len() int            { return len(h) }
func (h minHeap) Less(i, j int) bool  { return h[i].sim < h[j].sim }
func (h minHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x interface{}) { *h = append(*h, x.(heapItem)) }
func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

type worstHeap []heapItem

func (h worstHeap) Len() int            { return len(h) }
func (h worstHeap) Less(i, j int) bool  { return h[i].sim < h[j].sim }
func (h worstHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *worstHeap) Push(x interface{}) { *h = append(*h, x.(heapItem)) }
func (h *worstHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
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
			currObj = res[0].id
		}
	}

	items := h.internalSearchLayer(query, currObj, h.Ef, 0)
	results := make([]*vector.Vector, 0, k)
	for _, item := range items {
		node := h.Nodes[item.id]
		if node != nil && !node.Deleted {
			node.Vector.Score = item.sim
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
	binary.Write(f, binary.LittleEndian, h.nextID)

	for i := uint32(0); i < h.nextID; i++ {
		node, exists := h.Nodes[i]
		if !exists {

			binary.Write(f, binary.LittleEndian, i)
			binary.Write(f, binary.LittleEndian, true)    
			binary.Write(f, binary.LittleEndian, uint32(0)) 
			binary.Write(f, binary.LittleEndian, uint32(0)) 
			binary.Write(f, binary.LittleEndian, uint32(0)) 
			continue
		}

		binary.Write(f, binary.LittleEndian, node.ID)
		binary.Write(f, binary.LittleEndian, node.Deleted)

		idBytes := []byte(node.Vector.ID)
		binary.Write(f, binary.LittleEndian, uint32(len(idBytes)))
		f.Write(idBytes)

		binary.Write(f, binary.LittleEndian, uint32(len(node.Vector.Values)))
		binary.Write(f, binary.LittleEndian, node.Vector.Values)

		layerCount := uint32(len(node.Neighbors))
		binary.Write(f, binary.LittleEndian, layerCount)
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
	var nextID uint32

	if err := binary.Read(f, binary.LittleEndian, &entryNode); err != nil {
		return fmt.Errorf("reading entryNode: %w", err)
	}
	if err := binary.Read(f, binary.LittleEndian, &currentMax); err != nil {
		return fmt.Errorf("reading currentMax: %w", err)
	}
	if err := binary.Read(f, binary.LittleEndian, &nodeCount); err != nil {
		return fmt.Errorf("reading nodeCount: %w", err)
	}
	if err := binary.Read(f, binary.LittleEndian, &nextID); err != nil {
		return fmt.Errorf("reading nextID: %w", err)
	}

	h.EntryNode = entryNode
	h.CurrentMax = int(currentMax)
	h.nextID = nextID
	h.Nodes = make(map[uint32]*Node)

	for i := uint32(0); i < nextID; i++ {
		var nodeID uint32
		var isDeleted bool
		var idLen uint32

		if err := binary.Read(f, binary.LittleEndian, &nodeID); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("reading nodeID at slot %d: %w", i, err)
		}
		binary.Read(f, binary.LittleEndian, &isDeleted)
		binary.Read(f, binary.LittleEndian, &idLen)

		if idLen == 0 {

			var vecLen, layerCount uint32
			binary.Read(f, binary.LittleEndian, &vecLen)
			binary.Read(f, binary.LittleEndian, &layerCount)
			continue
		}

		idBytes := make([]byte, idLen)
		io.ReadFull(f, idBytes)

		var vecLen uint32
		binary.Read(f, binary.LittleEndian, &vecLen)
		values := make([]float32, vecLen)
		binary.Read(f, binary.LittleEndian, &values)

		var layerCount uint32
		binary.Read(f, binary.LittleEndian, &layerCount)

		neighbors := make([][]uint32, layerCount)
		for l := 0; l < int(layerCount); l++ {
			var neighborCount uint32
			binary.Read(f, binary.LittleEndian, &neighborCount)
			nbrs := make([]uint32, neighborCount)
			binary.Read(f, binary.LittleEndian, &nbrs)
			neighbors[l] = nbrs
		}

		h.Nodes[nodeID] = &Node{
			ID: nodeID,
			Vector: &vector.Vector{
				ID:     string(idBytes),
				Values: values,
			},
			Neighbors: neighbors,
			Deleted:   isDeleted,
		}
	}

	if nodeCount > 0 {
		if _, ok := h.Nodes[h.EntryNode]; !ok {
			return fmt.Errorf("entry node %d missing after load — index file may be corrupt", h.EntryNode)
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
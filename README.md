# GopherVectra

GopherVectra is a high-performance vector database engine built in Go. It implements a **Log-Structured Merge-Tree (LSM-Tree)** architecture for persistent storage and a **Hierarchical Navigable Small World (HNSW)** graph for fast, approximate nearest neighbor search.



## Core Engineering Features

* **HNSW Indexing:** Multi-layered graph structure providing $O(\log N)$ search complexity.
* **LSM-Tree Storage:** High-speed ingestion utilizing a Memtable and Write-Ahead Log (WAL).
* **Background Compaction:** A dedicated "Janitor" goroutine that merges multiple Level 0 SSTables into optimized Level 1 storage files to reduce File Descriptor overhead.
* **Distance Normalization:** Automatic scaling of 768D vectors to unit length (Magnitude = 1.0) to ensure stable Cosine Similarity.
* **Dual-Mode Search:** Supports standard HNSW search and an exact Brute Force fallback for Recall (accuracy) validation.

---

## Technical Architecture

### 1. The Write Path (Ingestion)
1.  **Normalization:** Incoming vectors are normalized to a unit sphere.
2.  **Durability (WAL):** Data is appended to `gopher.wal` before any RAM operations.
3.  **Memtable:** Vectors are held in a RAM buffer (Default limit: 50).
4.  **SSTable Flush:** When the Memtable is full, it flushes to a `level0_*.db` binary file.



### 2. Storage Maintenance
The **Compactor** monitors the storage directory. When it detects 10 or more Level 0 files, it performs a merge-sort operation to create a single Level 1 file, deleting the old segments to keep the search space clean.



### 3. The Search Path
The engine navigates the HNSW graph starting from the highest layer (coarse search) and moves down to Layer 0 (fine-grained search) to find the nearest neighbors.

---

## API Documentation

### Upsert Vector
**POST** `/upsert`
```json
{
  "id": "vector_01",
  "values": [0.12, -0.05, ... 768 dimensions]
}

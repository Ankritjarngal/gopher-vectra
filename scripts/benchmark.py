import requests
import time
import numpy as np
from concurrent.futures import ThreadPoolExecutor
import statistics

API_URL = "http://localhost:8080/search"
DIMENSIONS = 768
WARMUP_QUERIES = 50
NUM_QUERIES = 500
CONCURRENT_USERS = 10
K = 5

def normalize(v):
    arr = np.array(v)
    mag = np.linalg.norm(arr)
    return (arr / mag if mag > 0 else arr).tolist()

def generate_query():
    return normalize(np.random.uniform(-1, 1, DIMENSIONS).tolist())

# pre-generate all queries so generation time is not measured
queries = [generate_query() for _ in range(NUM_QUERIES + WARMUP_QUERIES)]

def single_query(idx):
    payload = {"values": queries[idx], "k": K}
    start = time.perf_counter()
    try:
        response = requests.post(API_URL, json=payload, timeout=5)
        latency = (time.perf_counter() - start) * 1000
        return latency if response.status_code == 200 else None
    except Exception:
        return None

def run_benchmark():
    print(f"Warming up with {WARMUP_QUERIES} queries...")
    with ThreadPoolExecutor(max_workers=CONCURRENT_USERS) as executor:
        list(executor.map(single_query, range(WARMUP_QUERIES)))

    print(f"Running benchmark: {NUM_QUERIES} queries, {CONCURRENT_USERS} concurrent users...")
    start_time = time.perf_counter()
    with ThreadPoolExecutor(max_workers=CONCURRENT_USERS) as executor:
        results = list(executor.map(single_query, range(WARMUP_QUERIES, WARMUP_QUERIES + NUM_QUERIES)))
    total_time = time.perf_counter() - start_time

    latencies = [r for r in results if r is not None]
    failed = NUM_QUERIES - len(latencies)

    if not latencies:
        print("All queries failed. Is the server running?")
        return

    print("\n" + "="*40)
    print("BENCHMARK RESULTS")
    print("="*40)
    print(f"Vectors in index:  (run GET /status to check)")
    print(f"Throughput:        {len(latencies) / total_time:.2f} QPS")
    print(f"Avg Latency:       {statistics.mean(latencies):.2f} ms")
    print(f"Median Latency:    {statistics.median(latencies):.2f} ms")
    print(f"P95 Latency:       {np.percentile(latencies, 95):.2f} ms")
    print(f"P99 Latency:       {np.percentile(latencies, 99):.2f} ms")
    print(f"Min/Max:           {min(latencies):.2f} ms / {max(latencies):.2f} ms")
    print(f"Success Rate:      {(len(latencies)/NUM_QUERIES)*100:.1f}%")
    print("="*40)

if __name__ == "__main__":
    run_benchmark()
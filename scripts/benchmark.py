import requests
import time
import numpy as np
from concurrent.futures import ThreadPoolExecutor
import statistics

# --- CONFIGURATION ---
API_URL = "http://localhost:8080/search"
DIMENSIONS = 768
NUM_QUERIES = 500
CONCURRENT_USERS = 10  # Simulating 10 users hitting the DB at once
K = 5

def generate_vector():
    return np.random.uniform(-1, 1, DIMENSIONS).tolist()

def single_query(_):
    payload = {
        "values": generate_vector(),
        "k": K
    }
    start = time.perf_counter()
    try:
        response = requests.post(API_URL, json=payload, timeout=5)
        latency = (time.perf_counter() - start) * 1000 # convert to ms
        return latency if response.status_code == 200 else None
    except Exception:
        return None

def run_benchmark():
    print(f"🚀 Starting Benchmark: {NUM_QUERIES} queries with {CONCURRENT_USERS} concurrent users...")
    
    start_time = time.perf_counter()
    
    with ThreadPoolExecutor(max_workers=CONCURRENT_USERS) as executor:
        results = list(executor.map(single_query, range(NUM_QUERIES)))
    
    end_time = time.perf_counter()
    total_time = end_time - start_time
    
    # Filter out failed requests
    latencies = [r for r in results if r is not None]
    failed = NUM_QUERIES - len(latencies)

    if not latencies:
        print("❌ All queries failed. Is the server running?")
        return

    # --- CALCULATE METRICS ---
    qps = len(latencies) / total_time
    avg_latency = statistics.mean(latencies)
    p95_latency = np.percentile(latencies, 95)
    p99_latency = np.percentile(latencies, 99)

    print("\n" + "="*40)
    print(f"📊 BENCHMARK RESULTS")
    print("="*40)
    print(f"Throughput:    {qps:.2f} QPS")
    print(f"Avg Latency:   {avg_latency:.2f} ms")
    print(f"P95 Latency:   {p95_latency:.2f} ms")
    print(f"P99 Latency:   {p99_latency:.2f} ms")
    print(f"Success Rate:  {((NUM_QUERIES-failed)/NUM_QUERIES)*100:.1f}%")
    print("="*40)

if __name__ == "__main__":
    run_benchmark()
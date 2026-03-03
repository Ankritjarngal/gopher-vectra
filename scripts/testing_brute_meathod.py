import requests
import json
import random

# Configuration
URL = "http://localhost:8080/search"
K = 5
# Generate a random 768D query vector
query_vector = [random.uniform(-1, 1) for _ in range(768)]

payload = {
    "values": query_vector,
    "k": K
}

def get_results(params=None):
    response = requests.post(URL, json=payload, params=params)
    if response.status_code == 200:
        return [res['id'] for res in response.json()]
    else:
        print(f"Error: {response.status_code}")
        return []

print("--- Starting Accuracy Test ---")

hnsw_ids = get_results()
print(f"HNSW Top {K}: {hnsw_ids}")

brute_ids = get_results(params={"method": "brute"})
print(f"Brute Force Top {K}: {brute_ids}")

matches = set(hnsw_ids).intersection(set(brute_ids))
recall = (len(matches) / K) * 100

print("-" * 30)
print(f"Matches found: {len(matches)} out of {K}")
print(f"Engine Recall Accuracy: {recall}%")
print("-" * 30)

if recall == 100:
    print("SUCCESS: HNSW is perfectly tuned for this dataset.")
elif recall >= 80:
    print("GOOD: HNSW is highly accurate.")
else:
    print("NOTICE: HNSW accuracy is low. Consider increasing EfConstruction or M.")
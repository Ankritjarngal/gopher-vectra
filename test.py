import requests
import random
import time

# Configuration
VECTRA_URL = "http://localhost:8080/upsert"
TOTAL_VECTORS = 600
DIMENSIONS = 768  # Set this to 768 for Gemini or 3 for manual testing

def generate_random_vector(dims):
    # Generates a list of random floats between -1.0 and 1.0
    return [random.uniform(-1.0, 1.0) for _ in range(dims)]

def run_bulk_ingest():
    print(f"Starting ingestion of {TOTAL_VECTORS} vectors...")
    start_time = time.time()
    
    for i in range(1, TOTAL_VECTORS + 1):
        doc_id = f"auto_vec_{i}"
        vector = generate_random_vector(DIMENSIONS)
        
        payload = {
            "id": doc_id,
            "values": vector,
            "metadata": {
                "source": "automated_script",
                "index": str(i),
                "timestamp": str(time.time())
            }
        }
        
        try:
            response = requests.post(VECTRA_URL, json=payload)
            if i % 50 == 0:
                print(f"Progress: {i}/{TOTAL_VECTORS} indexed...")
        except Exception as e:
            print(f"Error at index {i}: {e}")
            break
            
    end_time = time.time()
    print(f"Finished! Total time: {end_time - start_time:.2f} seconds.")

if __name__ == "__main__":
    run_bulk_ingest()
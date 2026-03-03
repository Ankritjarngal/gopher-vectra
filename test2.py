import requests
import random

def test_search():
    # Generate a random query vector (768 dims)
    query_vector = [random.uniform(-1.0, 1.0) for _ in range(768)]
    
    payload = {
        "values": query_vector,
        "k": 3
    }
    
    try:
        response = requests.post("http://localhost:8080/search", json=payload)
        print("--- Search Results ---")
        for res in response.json():
            print(f"ID: {res['id']} | Score: {res['score']:.4f}")
    except Exception as e:
        print(f"Error: {e}")

if __name__ == "__main__":
    test_search()
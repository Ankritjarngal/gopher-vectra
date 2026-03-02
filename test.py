import requests
import time
import os

URL = "http://localhost:8080/upsert"
STATUS_URL = "http://localhost:8080/status"

def check_db_files():
    return [f for f in os.listdir('.') if f.endswith('.db')]

def test_flush():
    print(f"Initial .db files: {check_db_files()}")
    
    for i in range(1, 7):
        payload = {
            "id": f"test_vec_{i}",
            "values": [0.1 * i, 0.2, 0.3],
            "metadata": {"info": f"vector number {i}"}
        }
        response = requests.post(URL, json=payload)
        print(f"Inserted {i}/6: Status {response.status_code}")
        
        if i == 5:
            print("Next insert should trigger flush...")
            
    time.sleep(1)
    new_files = check_db_files()
    if len(new_files) > 0:
        print(f"Success! .db files found: {new_files}")
    else:
        print("No .db files found. Check if server console shows 'Flushed memtable'.")

if __name__ == "__main__":
    test_flush()
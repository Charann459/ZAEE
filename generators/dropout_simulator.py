import json
import time
import random
from confluent_kafka import Producer

def main():
    p = Producer({'bootstrap.servers': 'localhost:9092'})
    
    print("Starting Dropout Simulator (Machine-Dropout)...")
    
    # Phase 1: Send noisy data to pass cold start
    print("Phase 1: Noisy data to pass cold start (100 samples)")
    for i in range(100):
        data = {
            "sensor_id": "Machine-Dropout",
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "fields": {
                "Stable_Sensor": 50.0 + random.gauss(0, 5) # High variance
            }
        }
        p.produce('zaee_ingest', json.dumps(data).encode('utf-8'))
        p.poll(0)
        time.sleep(0.1)
        
    p.flush()
    print("Cold start should be complete. Now sending FLAT data.")
    
    # Phase 2: Send perfectly flat data. The engine should suppress this via SDT,
    # except for the periodic Heartbeat! (Default 60s, so we run for 80s)
    for i in range(800): # 800 * 0.1s = 80 seconds
        data = {
            "sensor_id": "Machine-Dropout",
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "fields": {
                "Stable_Sensor": 50.0 # Perfectly flat
            }
        }
        p.produce('zaee_ingest', json.dumps(data).encode('utf-8'))
        p.poll(0)
        time.sleep(0.1)
        
    p.flush()
    print("Done!")

if __name__ == "__main__":
    main()

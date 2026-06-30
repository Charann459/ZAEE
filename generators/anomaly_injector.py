import json
import time
import random
from datetime import datetime, timezone

def generate_reading(sensor_id="temp_sensor_02", base_temp=85.0, variance=0.3):
    """Generates a reading, occasionally spiking to simulate an anomaly."""
    temp = base_temp + random.uniform(-variance, variance)
    
    # 5% chance of an anomaly spike
    if random.random() < 0.05:
        temp += random.uniform(10.0, 25.0)
        
    return {
        "sensor_id": sensor_id,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "fields": {
            "temperature": round(temp, 2)
        }
    }

def run_sensor(rate_hz=5):
    print(f"Starting anomaly sensor at {rate_hz}Hz. Press Ctrl+C to stop.")
    sleep_time = 1.0 / rate_hz
    
    try:
        while True:
            reading = generate_reading()
            print(json.dumps(reading))
            time.sleep(sleep_time)
    except KeyboardInterrupt:
        print("\nSensor stopped.")

if __name__ == "__main__":
    run_sensor()

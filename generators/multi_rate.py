import json
import time
import random
import threading
from datetime import datetime, timezone

def run_sensor(sensor_id, rate_hz, base_val, field_name, variance):
    """Runs a single sensor thread at a specific rate."""
    sleep_time = 1.0 / rate_hz
    try:
        while True:
            val = base_val + random.uniform(-variance, variance)
            reading = {
                "sensor_id": sensor_id,
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "fields": {
                    field_name: round(val, 2)
                }
            }
            print(json.dumps(reading))
            time.sleep(sleep_time)
    except KeyboardInterrupt:
        pass

def main():
    print("Starting multi-rate sensors. Press Ctrl+C to stop.")
    
    # 10Hz temperature sensor
    t1 = threading.Thread(target=run_sensor, args=("machine_01", 10, 90.0, "temperature", 0.5), daemon=True)
    # 15Hz pressure sensor
    t2 = threading.Thread(target=run_sensor, args=("machine_01", 15, 120.0, "pressure", 2.0), daemon=True)
    
    t1.start()
    t2.start()
    
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print("\nSensors stopped.")

if __name__ == "__main__":
    main()

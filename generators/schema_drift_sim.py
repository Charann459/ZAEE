import json
import time
import random
from datetime import datetime, timezone

def generate_initial_reading(sensor_id="drift_sensor_01", base_val=50.0):
    val = base_val + random.uniform(-1.0, 1.0)
    return {
        "sensor_id": sensor_id,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "fields": {
            "flow_rate": round(val, 2)
        }
    }

def generate_drifted_reading(sensor_id="drift_sensor_01", base_val=50.0):
    val = base_val + random.uniform(-1.0, 1.0)
    # Drift introduces a new field AND changes the type of the existing field
    return {
        "sensor_id": sensor_id,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "fields": {
            "flow_rate": str(round(val, 2)), # Drift: type changed to string
            "new_diagnostic_code": random.randint(100, 999) # Drift: new field
        }
    }

def run_sensor(rate_hz=5, drift_after_sec=10):
    print(f"Starting schema drift simulator at {rate_hz}Hz. Will drift after {drift_after_sec}s. Press Ctrl+C to stop.")
    sleep_time = 1.0 / rate_hz
    start_time = time.time()
    
    try:
        while True:
            elapsed = time.time() - start_time
            if elapsed > drift_after_sec:
                if elapsed - sleep_time <= drift_after_sec:
                     print(f"[{datetime.now(timezone.utc).isoformat()}] DRIFT OCCURRING NOW!")
                reading = generate_drifted_reading()
            else:
                reading = generate_initial_reading()
                
            print(json.dumps(reading))
            time.sleep(sleep_time)
    except KeyboardInterrupt:
        print("\nSensor stopped.")

if __name__ == "__main__":
    run_sensor()

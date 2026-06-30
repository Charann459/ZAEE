import json
import time
import random
from datetime import datetime, timezone

def generate_reading(sensor_id="pressure_relief_valve", value=150.0):
    return {
        "sensor_id": sensor_id,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "fields": {
            "pressure_psi": round(value, 1)
        }
    }

def run_sensor(check_hz=10, trigger_prob=0.01):
    print(f"Starting conditional sensor (Tier 1). Trigger probability {trigger_prob*100}% per cycle. Press Ctrl+C to stop.")
    sleep_time = 1.0 / check_hz
    
    try:
        while True:
            # Simulate a system pressure that occasionally spikes above threshold
            if random.random() < trigger_prob:
                # Sensor triggered!
                trigger_val = random.uniform(150.0, 180.0)
                reading = generate_reading(value=trigger_val)
                print(json.dumps(reading))
            
            # Note: unlike other generators, it does NOT print anything during normal operation
            time.sleep(sleep_time)
    except KeyboardInterrupt:
        print("\nSensor stopped.")

if __name__ == "__main__":
    run_sensor()

import json
import time
import random
from datetime import datetime, timezone

def generate_reading(sensor_id="temp_sensor_01", base_temp=90.0, variance=0.5):
    """Generates a steady sensor reading with slight noise."""
    temp = base_temp + random.uniform(-variance, variance)
    return {
        "sensor_id": sensor_id,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "fields": {
            "temperature": round(temp, 2)
        }
    }

def run_sensor(rate_hz=10):
    """Runs the sensor generator at the specified Hz."""
    print(f"Starting steady sensor at {rate_hz}Hz. Press Ctrl+C to stop.")
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

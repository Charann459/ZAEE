import json
import time
import random
from datetime import datetime, timezone

def generate_reading(sensor_id="vibration_sensor_01", base_vib=2.5, variance=0.1):
    vib = base_vib + random.uniform(-variance, variance)
    return {
        "sensor_id": sensor_id,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "fields": {
            "vibration_rms": round(vib, 3)
        }
    }

def run_sensor(rate_hz=10, active_duration_sec=5):
    print(f"Starting dropout simulator at {rate_hz}Hz. Will dropout after {active_duration_sec}s. Press Ctrl+C to stop.")
    sleep_time = 1.0 / rate_hz
    start_time = time.time()
    
    try:
        while True:
            if time.time() - start_time > active_duration_sec:
                print(f"[{datetime.now(timezone.utc).isoformat()}] Sensor dropping out...")
                # Sleep indefinitely to simulate dropout but keep process alive
                while True:
                    time.sleep(1)
            
            reading = generate_reading()
            print(json.dumps(reading))
            time.sleep(sleep_time)
    except KeyboardInterrupt:
        print("\nSensor stopped.")

if __name__ == "__main__":
    run_sensor()

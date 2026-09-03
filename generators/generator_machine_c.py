import sys
import os
import json
import asyncio
import argparse
from datetime import datetime, timezone
from kafka_helper import KafkaHelper

# Define the sensor groups by frequency
SENSORS_100HZ = ["PS1", "PS2", "PS3", "PS4", "PS5", "PS6", "EPS1"]
SENSORS_10HZ = ["FS1", "FS2"]
SENSORS_1HZ = ["TS1", "TS2", "TS3", "TS4", "VS1", "CE", "CP", "SE"]

def load_file_data(base_path, sensor_name):
    path = os.path.join(base_path, f"{sensor_name}.txt")
    if not os.path.exists(path):
        return []
    with open(path, 'r') as f:
        # Each line is a cycle (60 seconds)
        data = []
        for line in f:
            values = [float(v) for v in line.strip().split('\t') if v]
            data.append(values)
    return data

async def stream_frequency_group(group_name, sensors, data_dict, cycle_index, hz, speedup, kh):
    """Streams a group of sensors at their specific frequency for one cycle (60s)."""
    sleep_time = (1.0 / hz) / speedup
    num_samples = 60 * hz
    
    for i in range(num_samples):
        fields = {}
        for sensor in sensors:
            if sensor in data_dict and cycle_index < len(data_dict[sensor]):
                if i < len(data_dict[sensor][cycle_index]):
                    fields[sensor] = data_dict[sensor][cycle_index][i]
        
        if fields:
            payload = {
                "sensor_id": f"Machine-C_{group_name}",
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "fields": fields
            }
            kh.emit(payload)
            
        await asyncio.sleep(sleep_time)

async def main_async(speedup, kh):
    base_path = os.path.join(os.path.dirname(__file__), "..", "Sample Data", "Machine-C")
    
    print(f"[Machine-C] Loading dataset from {base_path}...", file=sys.stderr, flush=True)
    all_sensors = SENSORS_100HZ + SENSORS_10HZ + SENSORS_1HZ
    data_dict = {sensor: load_file_data(base_path, sensor) for sensor in all_sensors}
    
    num_cycles = min(len(data_dict["TS1"]), 5) if "TS1" in data_dict else 0
    
    print(f"[Machine-C] Loaded {num_cycles} cycles. Starting streaming at {speedup}x speed...", file=sys.stderr, flush=True)
    
    for cycle_index in range(num_cycles):
        print(f"[Machine-C] --- Starting Cycle {cycle_index + 1}/{num_cycles} ---", file=sys.stderr, flush=True)
        
        await asyncio.gather(
            stream_frequency_group("100Hz", SENSORS_100HZ, data_dict, cycle_index, 100, speedup, kh),
            stream_frequency_group("10Hz", SENSORS_10HZ, data_dict, cycle_index, 10, speedup, kh),
            stream_frequency_group("1Hz", SENSORS_1HZ, data_dict, cycle_index, 1, speedup, kh)
        )
        print(f"[Machine-C] --- Finished Cycle {cycle_index + 1} ---", file=sys.stderr, flush=True)

def main():
    parser = argparse.ArgumentParser(description="Machine C Stream Generator")
    parser.add_argument("--speedup", type=float, default=1.0, help="Speed multiplier (default: 1.0)")
    parser.add_argument("--kafka", action="store_true", help="Produce directly to Kafka instead of printing to stdout")
    args = parser.parse_args()

    kh = KafkaHelper(kafka_mode=args.kafka)

    try:
        asyncio.run(main_async(args.speedup, kh))
    except KeyboardInterrupt:
        print("\n[Machine-C] Stopped manually.", file=sys.stderr, flush=True)
    finally:
        kh.flush()

if __name__ == "__main__":
    main()

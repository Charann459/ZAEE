import os
import json
import asyncio
import argparse
from datetime import datetime, timezone

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
        # Parse all lines into a list of lists of floats
        data = []
        for line in f:
            values = [float(v) for v in line.strip().split('\t') if v]
            data.append(values)
    return data

async def stream_frequency_group(group_name, sensors, data_dict, cycle_index, hz, speedup):
    """Streams a group of sensors at their specific frequency for one cycle (60s)."""
    sleep_time = (1.0 / hz) / speedup
    num_samples = 60 * hz
    
    for i in range(num_samples):
        fields = {}
        for sensor in sensors:
            # Check if we have data for this sensor and cycle
            if sensor in data_dict and cycle_index < len(data_dict[sensor]):
                # Safe access in case row has missing values at the end
                if i < len(data_dict[sensor][cycle_index]):
                    fields[sensor] = data_dict[sensor][cycle_index][i]
        
        if fields:
            payload = {
                "sensor_id": f"Machine-C_{group_name}",
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "fields": fields
            }
            print(json.dumps(payload))
            
        await asyncio.sleep(sleep_time)

async def main_async(speedup):
    base_path = os.path.join(os.path.dirname(__file__), "..", "Sample Data", "Machine-C")
    
    print(f"[Machine-C] Loading dataset from {base_path}...")
    all_sensors = SENSORS_100HZ + SENSORS_10HZ + SENSORS_1HZ
    data_dict = {sensor: load_file_data(base_path, sensor) for sensor in all_sensors}
    
    # Assume all files have the same number of cycles (rows), use TS1 as baseline
    num_cycles = min(len(data_dict["TS1"]), 5) if "TS1" in data_dict else 0
    
    print(f"[Machine-C] Loaded {num_cycles} cycles. Starting streaming at {speedup}x speed...")
    
    for cycle_index in range(num_cycles):
        print(f"[Machine-C] --- Starting Cycle {cycle_index + 1}/{num_cycles} ---")
        
        # Run all frequency groups concurrently for this 60s cycle
        await asyncio.gather(
            stream_frequency_group("100Hz", SENSORS_100HZ, data_dict, cycle_index, 100, speedup),
            stream_frequency_group("10Hz", SENSORS_10HZ, data_dict, cycle_index, 10, speedup),
            stream_frequency_group("1Hz", SENSORS_1HZ, data_dict, cycle_index, 1, speedup)
        )
        print(f"[Machine-C] --- Finished Cycle {cycle_index + 1} ---")

def main():
    parser = argparse.ArgumentParser(description="Machine C Stream Generator")
    parser.add_argument("--speedup", type=float, default=1.0, help="Speed multiplier (default: 1.0)")
    args = parser.parse_args()

    try:
        asyncio.run(main_async(args.speedup))
    except KeyboardInterrupt:
        print("\n[Machine-C] Stopped manually.")

if __name__ == "__main__":
    main()

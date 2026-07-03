import os
import csv
import json
import time
import argparse
from datetime import datetime, timezone

def main():
    parser = argparse.ArgumentParser(description="Machine B Stream Generator")
    parser.add_argument("--speedup", type=float, default=1.0, help="Speed multiplier (default: 1.0)")
    parser.add_argument("--rate", type=float, default=2.0, help="Base Hz emission rate (default: 2.0)")
    args = parser.parse_args()

    data_path = os.path.join(os.path.dirname(__file__), "..", "Sample Data", "Machine-B", "Large_Industrial_Pump_Maintenance_Dataset.csv")
    
    if not os.path.exists(data_path):
        print(f"Error: Dataset not found at {data_path}")
        return

    base_sleep = 1.0 / args.rate
    actual_sleep = base_sleep / args.speedup

    print(f"[Machine-B] Starting generator at {args.rate}Hz (Speedup: {args.speedup}x)")

    try:
        with open(data_path, 'r') as f:
            reader = csv.DictReader(f)
            for row in reader:
                pump_id = row['Pump_ID']
                
                # Convert string values to float except ID and Flag
                fields = {k: float(v) for k, v in row.items() if k not in ('Pump_ID', 'Maintenance_Flag')}
                
                payload = {
                    "sensor_id": f"Machine-B_Pump_{pump_id}",
                    "timestamp": datetime.now(timezone.utc).isoformat(),
                    "fields": fields,
                    "target_label": {"Maintenance_Flag": int(row['Maintenance_Flag'])}
                }
                
                print(json.dumps(payload))
                time.sleep(actual_sleep)
                
    except KeyboardInterrupt:
        print("\n[Machine-B] Stopped manually.")

if __name__ == "__main__":
    main()

import os
import csv
import json
import time
import argparse
from datetime import datetime, timezone

def main():
    parser = argparse.ArgumentParser(description="Machine A Stream Generator")
    parser.add_argument("--speedup", type=float, default=1.0, help="Speed multiplier (default: 1.0)")
    parser.add_argument("--rate", type=float, default=5.0, help="Base Hz emission rate (default: 5.0)")
    args = parser.parse_args()

    data_path = os.path.join(os.path.dirname(__file__), "..", "Sample Data", "Machine-A", "Industrial_fault_detection.csv")
    
    if not os.path.exists(data_path):
        print(f"Error: Dataset not found at {data_path}")
        return

    base_sleep = 1.0 / args.rate
    actual_sleep = base_sleep / args.speedup

    print(f"[Machine-A] Starting generator at {args.rate}Hz (Speedup: {args.speedup}x)")

    try:
        with open(data_path, 'r') as f:
            reader = csv.DictReader(f)
            for row in reader:
                # Convert string values to appropriate types
                fields = {k: float(v) if '.' in v else int(v) for k, v in row.items() if k != 'Fault_Type'}
                # Split fields to simulate asynchronous multi-rate sensors for Fusion
                for k, v in fields.items():
                    payload = {
                        "sensor_id": "Machine-A",
                        "timestamp": datetime.now(timezone.utc).isoformat(),
                        "fields": {k: v},
                        "target_label": {"Fault_Type": int(row['Fault_Type'])}
                    }
                    print(json.dumps(payload))
                    time.sleep(0.01) # 10ms jitter between fields
                
                # Sleep the remainder of the actual_sleep
                time.sleep(max(0, actual_sleep - (len(fields) * 0.01)))
                
    except KeyboardInterrupt:
        print("\n[Machine-A] Stopped manually.")

if __name__ == "__main__":
    main()

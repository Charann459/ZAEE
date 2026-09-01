import sys
import os
import json
import time
import argparse
from datetime import datetime

def parse_secom_timestamp(ts_str):
    # e.g. "19/07/2008 11:55:00"
    ts_str = ts_str.strip('"')
    try:
        dt = datetime.strptime(ts_str, "%d/%m/%Y %H:%M:%S")
        return dt.isoformat() + "Z" # Append Z for UTC simplicity in this context
    except ValueError:
        return datetime.now().isoformat() + "Z"

def main():
    parser = argparse.ArgumentParser(description="SECOM Stream Generator")
    parser.add_argument("--speedup", type=float, default=1.0, help="Speed multiplier (default: 1.0)")
    parser.add_argument("--rate", type=float, default=1.0, help="Base Hz emission rate (default: 1.0)")
    args = parser.parse_args()

    base_path = os.path.join(os.path.dirname(__file__), "..", "Sample Data", "secom")
    data_file = os.path.join(base_path, "secom.data")
    labels_file = os.path.join(base_path, "secom_labels.data")
    
    if not os.path.exists(data_file) or not os.path.exists(labels_file):
        print(f"Error: SECOM Dataset not found at {base_path}", file=sys.stderr, flush=True)
        return

    base_sleep = 1.0 / args.rate
    actual_sleep = base_sleep / args.speedup

    print(f"[SECOM] Starting generator at {args.rate}Hz (Speedup: {args.speedup}x)", file=sys.stderr, flush=True)

    try:
        with open(data_file, 'r') as f_data, open(labels_file, 'r') as f_labels:
            for data_line, label_line in zip(f_data, f_labels):
                
                # Parse labels
                label_parts = label_line.strip().split(' ', 1)
                fail_flag = int(label_parts[0])
                timestamp_str = label_parts[1] if len(label_parts) > 1 else ""
                
                # Parse features
                features = data_line.strip().split(' ')
                fields = {}
                for i, val in enumerate(features):
                    if val == 'NaN':
                        fields[f"Feature_{i}"] = None
                    else:
                        fields[f"Feature_{i}"] = float(val)
                
                payload = {
                    "sensor_id": "SECOM_Process",
                    "timestamp": parse_secom_timestamp(timestamp_str),
                    "fields": fields,
                    "target_label": {"Fail": 1 if fail_flag == 1 else 0}
                }
                
                print(json.dumps(payload), flush=True)
                time.sleep(actual_sleep)
                
    except KeyboardInterrupt:
        print("\n[SECOM] Stopped manually.", file=sys.stderr, flush=True)

if __name__ == "__main__":
    main()

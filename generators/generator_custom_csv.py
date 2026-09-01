"""
generator_custom_csv.py
-----------------------
Universal adapter that converts ANY external CSV dataset into the ZAEE
JSON format and streams it to Kafka via stdout_to_kafka.py.

Works with datasets from Kaggle, UCI ML Repository, or any CSV source.

Usage:
    # First, see what columns are in your CSV:
    python generator_custom_csv.py --file mydata.csv --preview

    # Stream the full CSV to Kafka:
    python generator_custom_csv.py --file mydata.csv --sensor-id "my_sensor" | python stdout_to_kafka.py

    # Pick specific columns as sensor fields (skip ID/label columns):
    python generator_custom_csv.py --file mydata.csv --sensor-id "pump_01" --fields "temperature,pressure,vibration" | python stdout_to_kafka.py

    # Control streaming speed (default: 0.2s between rows):
    python generator_custom_csv.py --file mydata.csv --sensor-id "my_sensor" --delay 0.5 | python stdout_to_kafka.py

    # Use a column as the sensor_id (e.g. each machine has its own ID):
    python generator_custom_csv.py --file mydata.csv --sensor-col "machine_id" | python stdout_to_kafka.py

Example datasets this works with:
    - Kaggle: Any sensor/IoT CSV dataset
    - UCI: Predictive Maintenance, SECOM, Air Quality, etc.
    - Any CSV with numeric columns
"""

import csv
import json
import time
import argparse
import sys
import os
from datetime import datetime, timezone


def preview_csv(filepath: str):
    \"\"\"Print the first few rows and column info.\"\"\"
    with open(filepath, 'r', encoding='utf-8-sig') as f:
        reader = csv.DictReader(f)
        columns = reader.fieldnames
        rows = []
        for i, row in enumerate(reader):
            if i >= 5:
                break
            rows.append(row)

    print(f"\\nFile: {filepath}", file=sys.stderr, flush=True)
    print(f"Columns ({len(columns)} total):", file=sys.stderr, flush=True)
    for col in columns:
        sample = rows[0].get(col, '') if rows else ''
        print(f"  - {col!r:<35} sample: {sample!r}", file=sys.stderr, flush=True)

    print(f"\\nFirst row as ZAEE payload (auto-detected numeric fields):", file=sys.stderr, flush=True)
    if rows:
        numeric_fields = {}
        for k, v in rows[0].items():
            try:
                numeric_fields[k] = float(v)
            except (ValueError, TypeError):
                pass
        print(json.dumps({
            "sensor_id": "YOUR_SENSOR_NAME",
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "fields": numeric_fields
        }, indent=2))

    print(f"\\nRun command:", file=sys.stderr, flush=True)
    print(f'  python generator_custom_csv.py --file "{filepath}" --sensor-id "my_sensor" | python stdout_to_kafka.py', file=sys.stderr, flush=True)


def stream_csv(filepath, sensor_id, sensor_col, field_names, delay, max_rows):
    with open(filepath, 'r', encoding='utf-8-sig') as f:
        reader = csv.DictReader(f)
        all_cols = reader.fieldnames or []

        # Determine which columns to use as fields
        if field_names:
            use_cols = [c.strip() for c in field_names.split(',')]
        else:
            # Auto-detect: use all numeric columns
            use_cols = None  # decided per-row

        row_count = 0
        emitted = 0

        for row in reader:
            if max_rows and row_count >= max_rows:
                break
            row_count += 1

            # Determine sensor_id for this row
            sid = row.get(sensor_col, sensor_id) if sensor_col else sensor_id

            # Build fields dict from numeric columns
            fields = {}
            cols_to_try = use_cols if use_cols else all_cols
            for col in cols_to_try:
                val = row.get(col, '').strip()
                if not val:
                    continue
                # Skip the sensor_col itself
                if sensor_col and col == sensor_col:
                    continue
                try:
                    fields[col] = float(val)
                except ValueError:
                    # Non-numeric: send as string (engine handles schema drift)
                    fields[col] = val

            if not fields:
                continue  # skip empty rows

            payload = {
                "sensor_id": str(sid),
                "timestamp": datetime.now(timezone.utc).isoformat(),
                "fields": fields
            }
            print(json.dumps(payload), flush=True)
            emitted += 1

            if delay > 0:
                time.sleep(delay)

    print(f"[Custom CSV] Done. Streamed {emitted} rows from '{filepath}'.", file=sys.stderr)


def main():
    parser = argparse.ArgumentParser(
        description='Stream any external CSV dataset into ZAEE via Kafka'
    )
    parser.add_argument('--file', required=True,
                        help='Path to your CSV file (Kaggle, UCI, etc.)')
    parser.add_argument('--sensor-id', default='external_sensor',
                        help='Name to use as sensor_id (default: external_sensor)')
    parser.add_argument('--sensor-col', default=None,
                        help='CSV column to use as sensor_id (for multi-machine datasets)')
    parser.add_argument('--fields', default=None,
                        help='Comma-separated column names to use as fields. '
                             'Default: auto-detect all numeric columns')
    parser.add_argument('--delay', type=float, default=0.2,
                        help='Seconds between each row (default: 0.2)')
    parser.add_argument('--max-rows', type=int, default=None,
                        help='Maximum rows to stream (default: all rows)')
    parser.add_argument('--preview', action='store_true',
                        help='Print column info and sample payload, then exit')
    args = parser.parse_args()

    if not os.path.exists(args.file):
        print(f"[Custom CSV] ERROR: File not found: {args.file}", file=sys.stderr)
        sys.exit(1)

    if args.preview:
        preview_csv(args.file)
        return

    total_rows = sum(1 for _ in open(args.file, encoding='utf-8-sig')) - 1
    limit = args.max_rows or total_rows
    print(f"[Custom CSV] File: {args.file}", file=sys.stderr)
    print(f"[Custom CSV] Sensor ID: {args.sensor_col or args.sensor_id}", file=sys.stderr)
    print(f"[Custom CSV] Streaming {limit} of {total_rows} rows at {args.delay}s delay...", file=sys.stderr)

    stream_csv(args.file, args.sensor_id, args.sensor_col, args.fields, args.delay, args.max_rows)


if __name__ == '__main__':
    try:
        main()
    except KeyboardInterrupt:
        print('\\n[Custom CSV] Stopped manually.', file=sys.stderr)

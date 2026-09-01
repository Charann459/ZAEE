"""
generator_chaos_demo.py
-----------------------
A spectacular, all-in-one chaos generator for panel interviews.
This script continuously fires a mix of normal and anomalous data
to instantly light up the dashboard with all 5 flag colors:
- Red (Dropout)
- Amber (Field Missing)
- Blue (LOCF Gap Fill)
- Purple (Schema Drift)
- Green (Schema Promoted)

Usage:
    python generator_chaos_demo.py | python stdout_to_kafka.py
"""

import json
import time
import sys
import random
from datetime import datetime, timezone

def emit(payload: dict):
    print(json.dumps(payload), flush=True)

def now():
    return datetime.now(timezone.utc).isoformat()

def main():
    print("[Chaos Demo] Starting spectacular panel demo generator...", file=sys.stderr)
    
    # 1. Normal Data (Baseline)
    for i in range(10):
        emit({
            "sensor_id": "panel_machine_01",
            "timestamp": now(),
            "fields": {"temp": 45.0, "pressure": 100.0, "vibration": 0.5}
        })
        time.sleep(0.1)

    print("[Chaos Demo] Triggering LOCF Gap Fill (Blue) & Field Unavailable (Amber)...", file=sys.stderr)
    for i in range(15):
        # Drop pressure to trigger LOCF, then drop it long enough to trigger Unavailable
        emit({
            "sensor_id": "panel_machine_01",
            "timestamp": now(),
            "fields": {"temp": 45.5, "vibration": 0.52} # pressure missing!
        })
        time.sleep(0.5)

    print("[Chaos Demo] Triggering Schema Drift (Purple)...", file=sys.stderr)
    for i in range(10):
        # Change vibration from float to string
        emit({
            "sensor_id": "panel_machine_01",
            "timestamp": now(),
            "fields": {"temp": 46.0, "pressure": 101.0, "vibration": "HIGH"}
        })
        time.sleep(0.5)

    print("[Chaos Demo] Triggering Schema Promoted (Green)...", file=sys.stderr)
    # 25 samples of string type promotes the schema
    for i in range(26):
        emit({
            "sensor_id": "panel_machine_01",
            "timestamp": now(),
            "fields": {"temp": 46.0, "pressure": 101.0, "vibration": "HIGH"}
        })
        time.sleep(0.2)
        
    print("[Chaos Demo] Triggering Sensor Dropout (Red) - Engine will flag after 10s...", file=sys.stderr)
    emit({
        "sensor_id": "panel_dropout_sensor",
        "timestamp": now(),
        "fields": {"heartbeat": 1}
    })
    
    print("[Chaos Demo] Demo data fully injected! Keep this running for a few seconds to let engine process.", file=sys.stderr)
    time.sleep(15) # Wait for dropout timeout to trigger

if __name__ == '__main__':
    main()

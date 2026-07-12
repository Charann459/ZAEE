import json
import time
import argparse
from datetime import datetime, timezone

def emit_payload(sensor_id, f1_value):
    payload = {
        "sensor_id": sensor_id,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "fields": {
            "f1": f1_value,
            "f2": 5.0 # Stable reference field
        }
    }
    print(json.dumps(payload))
    time.sleep(0.1) # 10Hz

def main(speedup):
    sensor_id = "Drift-Sim"
    
    print(f"[Type Oscillation Sim] Starting. Phase 1: Main Cold Start (60 samples of float)", flush=True)
    for i in range(60):
        emit_payload(sensor_id, 10.0 + (i * 0.1))
        
    print(f"[Type Oscillation Sim] Phase 2: Stable Operation (20 samples of float)", flush=True)
    for i in range(20):
        emit_payload(sensor_id, 16.0)
        
    print(f"[Type Oscillation Sim] Phase 3: Oscillating Drift (String -> Bool -> String -> Bool)", flush=True)
    # The checklist is 25 samples. We want to oscillate before it hits 25, resetting it.
    for i in range(10):
        emit_payload(sensor_id, "error_state")
    for i in range(10):
        emit_payload(sensor_id, True)
    for i in range(10):
        emit_payload(sensor_id, "error_state")
        
    print(f"[Type Oscillation Sim] Phase 4: Settling on Bool (30 samples, should trigger promotion)", flush=True)
    for i in range(30):
        emit_payload(sensor_id, True)
        
    print(f"[Type Oscillation Sim] Phase 5: Stable Operation on new type (Bool)", flush=True)
    for i in range(20):
        emit_payload(sensor_id, True)
        
    print("[Type Oscillation Sim] Simulation complete.", flush=True)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Type Oscillation Simulator")
    parser.add_argument("--speedup", type=float, default=1.0, help="Speed multiplier (ignored for this sim)")
    args = parser.parse_args()
    
    try:
        main(args.speedup)
    except KeyboardInterrupt:
        print("\n[Type Oscillation Sim] Stopped manually.")

"""
generator_scenario.py
---------------------
Streams the zaee_scenario_dataset.json file to Kafka (or stdout).
Each scenario's input payload(s) are sent in order, with a configurable
delay between them so the engine has time to process each one.

Usage:
    # Stream to Kafka (recommended for demo):
    python generator_scenario.py | python stdout_to_kafka.py

    # Stream to stdout only (for debugging):
    python generator_scenario.py

    # Replay a specific category only:
    python generator_scenario.py --category "Schema Drift"

    # Slow down between scenarios (default 1.0s):
    python generator_scenario.py --delay 2.0
"""

import json
import time
import argparse
import sys
import os
from datetime import datetime, timezone

DATASET_PATH = os.path.join(os.path.dirname(__file__), '..', 'zaee_scenario_dataset.json')

def emit(payload: dict):
    print(json.dumps(payload), flush=True)

def normalise_timestamp(payload: dict) -> dict:
    payload = dict(payload)
    payload['timestamp'] = datetime.now(timezone.utc).isoformat()
    return payload

def main():
    parser = argparse.ArgumentParser(description='ZAEE Scenario Dataset Generator')
    parser.add_argument('--delay', type=float, default=1.0,
                        help='Seconds to pause between each scenario (default: 1.0)')
    parser.add_argument('--category', type=str, default=None,
                        help='Only replay scenarios from this category (case-insensitive)')
    parser.add_argument('--list', action='store_true',
                        help='List all scenario names and categories, then exit')
    args = parser.parse_args()

    with open(DATASET_PATH, 'r') as f:
        dataset = json.load(f)

    scenarios = dataset.get('scenarios', [])

    if args.list:
        print(f"\nDataset: {dataset['dataset_name']} (v{dataset['version']})", file=sys.stderr, flush=True)
        print(f"Total scenarios: {dataset['total_scenarios']}\n", file=sys.stderr, flush=True)
        for s in scenarios:
            print(f"  [{s['category']}]  {s['scenario']}", file=sys.stderr, flush=True)
        print(, file=sys.stderr, flush=True)
        return

    if args.category:
        filter_cat = args.category.lower()
        scenarios = [s for s in scenarios if filter_cat in s['category'].lower()]
        if not scenarios:
            print(f"[Scenario Gen] ERROR: No scenarios found for category '{args.category}'", file=sys.stderr)
            sys.exit(1)

    total = len(scenarios)
    print(f"[Scenario Gen] Starting: {total} scenarios | delay={args.delay}s", file=sys.stderr)

    for idx, scenario in enumerate(scenarios, 1):
        name = scenario['scenario']
        category = scenario['category']
        desc = scenario.get('description', '')
        inp = scenario.get('input')

        print(f"[Scenario Gen] [{idx}/{total}] {name} ({category})", file=sys.stderr)
        print(f"[Scenario Gen]   {desc[:90]}{'...' if len(desc) > 90 else ''}", file=sys.stderr)

        if isinstance(inp, list):
            for payload in inp:
                emit(normalise_timestamp(payload))
                time.sleep(0.1)
        elif isinstance(inp, dict):
            emit(normalise_timestamp(inp))
        else:
            print(f"[Scenario Gen]   WARNING: Skipping - no valid input", file=sys.stderr)
            continue

        time.sleep(args.delay)

    print(f"[Scenario Gen] Done! All {total} scenarios streamed.", file=sys.stderr)

if __name__ == '__main__':
    try:
        main()
    except KeyboardInterrupt:
        print('\n[Scenario Gen] Stopped manually.', file=sys.stderr)

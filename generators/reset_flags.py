"""
reset_flags.py
--------------
Full demo reset: clears flags from PostgreSQL, flushes Redis engine state,
and resets Cloud Impact stats on the dashboard.

Run this before EVERY fresh demo run.

Usage:
    python reset_flags.py
"""

import subprocess
import sys
import urllib.request
import urllib.error

def main():
    # ── Step 1: Reset Cloud Impact stats on dashboard ──────────────────────
    print('[Reset] (1/3) Resetting Cloud Impact stats on dashboard...')
    try:
        req = urllib.request.Request('http://localhost:8000/api/stats/reset', method='POST')
        urllib.request.urlopen(req)
        print('[Reset]       Done.')
    except urllib.error.URLError as e:
        print(f'[Reset]       WARNING: Could not reach dashboard API: {e}')

    # ── Step 2: Flush Redis to clear engine sensor state ───────────────────
    # This prevents the ZAEE engine from re-emitting sensor_dropout flags
    # for sensors from a previous run (e.g., Machine-B flags appearing during Machine-C run).
    print('[Reset] (2/3) Flushing Redis engine state (clears old sensor baselines)...')
    redis_result = subprocess.run(
        ['docker', 'exec', 'redis', 'redis-cli', 'FLUSHDB'],
        capture_output=True, text=True
    )
    if redis_result.returncode == 0:
        print('[Reset]       Done.')
    else:
        print(f'[Reset]       WARNING: Could not flush Redis: {redis_result.stderr}')

    # ── Step 3: Clear flags from PostgreSQL ────────────────────────────────
    print('[Reset] (3/3) Counting unacknowledged flags in database...')

    count_result = subprocess.run(
        [
            'docker', 'exec', 'postgres',
            'psql', '-U', 'zaee', '-d', 'zaee',
            '-t', '-c', 'SELECT COUNT(*) FROM flags WHERE acknowledged = false;'
        ],
        capture_output=True, text=True
    )

    if count_result.returncode != 0:
        print('[Reset] ERROR: Could not connect to PostgreSQL.')
        print('        Make sure Docker is running: docker-compose up -d')
        print(count_result.stderr)
        sys.exit(1)

    count = count_result.stdout.strip()
    print(f'[Reset]       Found {count} unacknowledged flags.')

    if count == '0':
        print('[Reset] Nothing to clear in database. Dashboard is fully clean!')
        return

    confirm = input(f'[Reset] Delete all {count} unacknowledged flags? (yes/no): ').strip().lower()
    if confirm != 'yes':
        print('[Reset] Cancelled.')
        return

    delete_result = subprocess.run(
        [
            'docker', 'exec', 'postgres',
            'psql', '-U', 'zaee', '-d', 'zaee',
            '-c', 'DELETE FROM flags WHERE acknowledged = false;'
        ],
        capture_output=True, text=True
    )

    if delete_result.returncode == 0:
        print(f'[Reset] Done! Cleared {count} flags.')
        print('[Reset] ✅ Full reset complete — engine state, stats, and flags are clean.')
    else:
        print('[Reset] ERROR during delete:')
        print(delete_result.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()

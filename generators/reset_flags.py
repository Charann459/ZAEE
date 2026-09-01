"""
reset_flags.py
--------------
Clears all unacknowledged flags from the PostgreSQL database.
Run this before a fresh demo to start with a clean slate.

Usage:
    python reset_flags.py
"""

import subprocess
import sys
import urllib.request
import urllib.error

def main():
    print('[Reset] Calling dashboard API to reset Cloud Impact stats...')
    try:
        req = urllib.request.Request('http://localhost:8000/api/stats/reset', method='POST')
        urllib.request.urlopen(req)
        print('[Reset] Cloud Impact stats reset successfully.')
    except urllib.error.URLError as e:
        print(f'[Reset] WARNING: Could not connect to dashboard API to reset stats: {e}')

    print('[Reset] Counting unacknowledged flags...')

    # Use docker exec to run psql - no Python DB drivers needed
    count_result = subprocess.run(
        [
            'docker', 'exec', 'postgres',
            'psql', '-U', 'zaee', '-d', 'zaee',
            '-t', '-c', 'SELECT COUNT(*) FROM flags WHERE acknowledged = false;'
        ],
        capture_output=True, text=True
    )

    if count_result.returncode != 0:
        print(f'[Reset] ERROR: Could not connect to PostgreSQL.')
        print(f'        Make sure Docker is running: docker-compose up -d')
        print(count_result.stderr)
        sys.exit(1)

    count = count_result.stdout.strip()
    print(f'[Reset] Found {count} unacknowledged flags.')

    if count == '0':
        print('[Reset] Nothing to clear. Dashboard is already clean!')
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
        print(f'[Reset] Done! Cleared {count} flags. Dashboard is now clean for a fresh demo.')
    else:
        print(f'[Reset] ERROR during delete:')
        print(delete_result.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()

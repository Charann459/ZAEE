"""
reset_flags.py
--------------
Clears all unacknowledged flags from the PostgreSQL database.
Run this before a fresh demo to start with a clean slate.

Usage (from the generators/ or project root):
    python reset_flags.py
"""

import asyncio
import asyncpg
import os

DB_URL = os.getenv('DB_URL', 'postgresql://zaee:zaee_password@localhost:5432/zaee')

async def main():
    print('[Reset] Connecting to PostgreSQL...')
    conn = await asyncpg.connect(DB_URL)
    
    count = await conn.fetchval("SELECT COUNT(*) FROM flags WHERE acknowledged = false")
    print(f'[Reset] Found {count} unacknowledged flags.')
    
    if count == 0:
        print('[Reset] Nothing to clear. Dashboard is already clean!')
        await conn.close()
        return

    confirm = input(f'[Reset] Delete all {count} unacknowledged flags? (yes/no): ').strip().lower()
    if confirm != 'yes':
        print('[Reset] Cancelled.')
        await conn.close()
        return

    deleted = await conn.execute("DELETE FROM flags WHERE acknowledged = false")
    print(f'[Reset] Done! Cleared {count} flags. Dashboard is now clean for a fresh demo.')
    await conn.close()

if __name__ == '__main__':
    asyncio.run(main())

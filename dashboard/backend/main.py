import os
import json
import asyncio
from datetime import datetime, timezone
from typing import Optional, List
from collections import deque

from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import StreamingResponse
from fastapi.staticfiles import StaticFiles
from pydantic import BaseModel
import asyncpg
from aiokafka import AIOKafkaConsumer

# Configuration
KAFKA_BROKER = os.getenv("KAFKA_BROKER", "localhost:9092")
KAFKA_TOPIC = os.getenv("KAFKA_TOPIC", "zaee_output")
DB_URL = os.getenv("DB_URL", "postgresql://zaee:zaee_password@localhost:5432/zaee")

class EventBus:
    def __init__(self):
        self.subscribers: List[asyncio.Queue] = []

    async def publish(self, event: dict):
        for q in self.subscribers:
            await q.put(event)

    def subscribe(self) -> asyncio.Queue:
        q = asyncio.Queue()
        self.subscribers.append(q)
        return q

    def unsubscribe(self, q: asyncio.Queue):
        if q in self.subscribers:
            self.subscribers.remove(q)

event_bus = EventBus()
db_pool = None

import time

# Stats shared state
dashboard_start_time = time.time()
stats_ingest_deque = deque(maxlen=10)
stats_output_deque = deque(maxlen=10)

current_sec_ingest = 0
current_sec_output = 0

total_ingest_count = 0
total_output_count = 0

app = FastAPI(title="ZAEE Dashboard API")

class AcknowledgeRequest(BaseModel):
    user: str
    note: str

@app.on_event("startup")
async def startup_event():
    global db_pool
    print("[Dashboard] Connecting to PostgreSQL...")
    db_pool = await asyncpg.create_pool(DB_URL)
    
    print("[Dashboard] Starting Kafka Consumer background task...")
    asyncio.create_task(kafka_consumer_task())
    asyncio.create_task(kafka_ingest_stats_task())
    asyncio.create_task(kafka_output_stats_task())
    asyncio.create_task(stats_publisher_task())

@app.on_event("shutdown")
async def shutdown_event():
    if db_pool:
        await db_pool.close()

async def kafka_ingest_stats_task():
    global current_sec_ingest, total_ingest_count
    while True:
        consumer = AIOKafkaConsumer(
            "zaee_ingest",
            bootstrap_servers=KAFKA_BROKER,
            group_id="zaee_dashboard_stats_ingest",
            auto_offset_reset="latest"
        )
        try:
            await consumer.start()
            print("[Dashboard] Ingest stats consumer connected.")
            async for msg in consumer:
                current_sec_ingest += 1
                total_ingest_count += 1
        except Exception as e:
            print(f"[Dashboard] Ingest stats lost connection: {e}. Reconnecting in 5s...")
        finally:
            try:
                await consumer.stop()
            except Exception:
                pass
        await asyncio.sleep(5)

async def kafka_output_stats_task():
    global current_sec_output, total_output_count
    while True:
        consumer = AIOKafkaConsumer(
            KAFKA_TOPIC,
            bootstrap_servers=KAFKA_BROKER,
            group_id="zaee_dashboard_stats_output",
            auto_offset_reset="latest"
        )
        try:
            await consumer.start()
            print("[Dashboard] Output stats consumer connected.")
            async for msg in consumer:
                current_sec_output += 1
                total_output_count += 1
        except Exception as e:
            print(f"[Dashboard] Output stats lost connection: {e}. Reconnecting in 5s...")
        finally:
            try:
                await consumer.stop()
            except Exception:
                pass
        await asyncio.sleep(5)

async def stats_publisher_task():
    global current_sec_ingest, current_sec_output
    while True:
        await asyncio.sleep(1.0)
        
        stats_ingest_deque.append(current_sec_ingest)
        stats_output_deque.append(current_sec_output)
        
        current_sec_ingest = 0
        current_sec_output = 0
        
        ingest_rate = sum(stats_ingest_deque) / len(stats_ingest_deque) if len(stats_ingest_deque) > 0 else 0
        output_rate = sum(stats_output_deque) / len(stats_output_deque) if len(stats_output_deque) > 0 else 0
        
        event = {
            "type": "stats_update",
            "data": {
                "uptime": time.time() - dashboard_start_time,
                "ingest_rate": round(ingest_rate, 2),
                "output_rate": round(output_rate, 2),
                "total_ingest": total_ingest_count,
                "total_output": total_output_count
            }
        }
        await event_bus.publish(event)

async def kafka_consumer_task():
    while True:
        consumer = AIOKafkaConsumer(
            KAFKA_TOPIC,
            bootstrap_servers=KAFKA_BROKER,
            group_id=f"zaee_dashboard_group_{time.time()}",
            auto_offset_reset="latest"  # Only process NEW messages - DB is source of truth for history
        )
        try:
            await consumer.start()
            print("[Dashboard] Flag consumer connected.")
            async for msg in consumer:
                try:
                    payload = json.loads(msg.value.decode("utf-8"))
                    flags = payload.get("flags")
                    if not flags:
                        continue
                    
                    sensor_id = payload.get("sensor_id")
                    timestamp = payload.get("timestamp")
                    if not timestamp:
                        timestamp = datetime.now(timezone.utc).isoformat()

                    for field_name, flag_type in flags.items():
                        # Upsert flag into PostgreSQL
                        query = """
                            INSERT INTO flags (sensor_id, field_name, flag_type, message, first_detected_at, last_detected_at, acknowledged)
                            VALUES ($1, $2, $3, $4, $5, $5, false)
                            ON CONFLICT (sensor_id, field_name, flag_type) WHERE acknowledged = false
                            DO UPDATE SET last_detected_at = EXCLUDED.last_detected_at
                            RETURNING id, first_detected_at, last_detected_at;
                        """
                        async with db_pool.acquire() as conn:
                            row = await conn.fetchrow(query, sensor_id, field_name, flag_type, flag_type, datetime.fromisoformat(timestamp.replace('Z', '+00:00')))
                            
                            # Publish to SSE
                            event = {
                                "type": "flag_upsert",
                                "data": {
                                    "id": row["id"],
                                    "sensor_id": sensor_id,
                                    "field_name": field_name,
                                    "flag_type": flag_type,
                                    "message": flag_type,
                                    "first_detected_at": row["first_detected_at"].isoformat(),
                                    "last_detected_at": row["last_detected_at"].isoformat(),
                                    "acknowledged": False
                                }
                            }
                            await event_bus.publish(event)
                except Exception as e:
                    print(f"[Dashboard] Error processing Kafka message: {e}")
        except Exception as e:
            print(f"[Dashboard] Flag consumer lost connection: {e}. Reconnecting in 5s...")
        finally:
            try:
                await consumer.stop()
            except Exception:
                pass
        await asyncio.sleep(5)

@app.get("/api/flags")
async def get_flags(status: Optional[str] = None):
    async with db_pool.acquire() as conn:
        if status == "unacknowledged":
            rows = await conn.fetch("SELECT * FROM flags WHERE acknowledged = false ORDER BY last_detected_at DESC")
        elif status == "acknowledged":
            rows = await conn.fetch("SELECT * FROM flags WHERE acknowledged = true ORDER BY acknowledged_at DESC")
        else:
            rows = await conn.fetch("SELECT * FROM flags ORDER BY last_detected_at DESC")
            
        return [dict(row) for row in rows]

@app.post("/api/flags/{flag_id}/acknowledge")
async def acknowledge_flag(flag_id: int, req: AcknowledgeRequest):
    if not req.user or not req.user.strip():
        raise HTTPException(status_code=400, detail="User is required and cannot be empty")
    if not req.note or not req.note.strip():
        raise HTTPException(status_code=400, detail="Note is required and cannot be empty")
        
    query = """
        UPDATE flags
        SET acknowledged = true,
            acknowledged_by = $1,
            note = $2,
            acknowledged_at = NOW()
        WHERE id = $3 AND acknowledged = false
        RETURNING *;
    """
    async with db_pool.acquire() as conn:
        row = await conn.fetchrow(query, req.user.strip(), req.note.strip(), flag_id)
        
        if not row:
            raise HTTPException(status_code=404, detail="Flag not found or already acknowledged")
            
        # Publish acknowledgment event to SSE
        event = {
            "type": "flag_acknowledged",
            "data": dict(row)
        }
        # Convert datetime to isoformat for JSON serialization
        for key, val in event["data"].items():
            if isinstance(val, datetime):
                event["data"][key] = val.isoformat()
                
        await event_bus.publish(event)
        
        return event["data"]

@app.post("/api/stats/reset")
async def reset_stats():
    global current_sec_ingest, current_sec_output, total_ingest_count, total_output_count, dashboard_start_time
    
    current_sec_ingest = 0
    current_sec_output = 0
    total_ingest_count = 0
    total_output_count = 0
    dashboard_start_time = time.time()
    
    stats_ingest_deque.clear()
    stats_output_deque.clear()
    
    # Broadcast to all connected browsers to clear their in-memory flag state
    await event_bus.publish({"type": "flags_reset"})
    
    return {"status": "success", "message": "Cloud impact stats reset and browser state cleared."}

@app.get("/api/events")
async def sse_events(request: Request):
    async def event_stream():
        q = event_bus.subscribe()
        try:
            while True:
                # Wait for event or client disconnect
                event = await q.get()
                yield f"data: {json.dumps(event)}\n\n"
        except asyncio.CancelledError:
            pass
        finally:
            event_bus.unsubscribe(q)

    return StreamingResponse(event_stream(), media_type="text/event-stream")

# Trigger reload
# Mount frontend
app.mount("/", StaticFiles(directory="../frontend", html=True), name="frontend")
# Reloading after system restart

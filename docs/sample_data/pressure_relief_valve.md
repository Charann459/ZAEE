# Sensor: `pressure_relief_valve` (Conditional / Tier 1 Sensor)

## Overview
This simulates a safety-critical conditional sensor (Tier 1). By definition, this type of sensor only emits data when a threshold is breached. It does not have a "normal" baseline. It is used to test Tier 1 routing, where the engine must bypass deadband/baseline checks and instantly pass the payload through.

## Sensor Details
- **Sensor Name/ID**: `pressure_relief_valve`
- **Machine/Asset Type**: High Pressure Pipeline
- **Data Type**: Float
- **Firing Rate**: Sporadic (Checks at 10Hz internally, but only triggers with a 1% probability per check).
- **Trigger Range**: 150.0 - 180.0 PSI
- **Behavior**: Completely silent during normal operation. Randomly (approx. once every 10 seconds), it will trigger and emit a high-pressure reading. 

## Timestamping
Timestamps are in ISO-8601 UTC format (e.g., `2026-06-30T10:15:30.123456+00:00`).

## Sample JSON Structure

**Triggered Reading:**
```json
{
  "sensor_id": "pressure_relief_valve",
  "timestamp": "2026-06-30T10:15:30.100000+00:00",
  "fields": {
    "pressure_psi": 165.4
  }
}
```
*(No standard readings are emitted)*

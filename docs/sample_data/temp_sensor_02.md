# Sensor: `temp_sensor_02` (Anomaly Injector)

## Overview
This simulates a temperature sensor that operates normally most of the time but occasionally experiences massive spikes. It is designed to test the Engine's baseline outlier detection during Cold Start and anomaly pass-through during normal operation.

## Sensor Details
- **Sensor Name/ID**: `temp_sensor_02`
- **Machine/Asset Type**: Industrial Furnace / Boiler
- **Data Type**: Float
- **Firing Rate**: 5Hz (5 readings per second)
- **Normal Range**: 84.7 - 85.3 °C (Base 85.0, Variance ±0.3)
- **Anomaly Range**: 94.7 - 110.3 °C (+10.0 to +25.0 increase)
- **Behavior**: Emits steady data, but has a 5% probability per cycle to inject a massive temperature spike simulating an anomaly or malfunction.

## Timestamping
Timestamps are in ISO-8601 UTC format (e.g., `2026-06-30T10:15:30.123456+00:00`).

## Sample JSON Structure

**Normal Reading:**
```json
{
  "sensor_id": "temp_sensor_02",
  "timestamp": "2026-06-30T10:15:30.200000+00:00",
  "fields": {
    "temperature": 85.12
  }
}
```

**Anomaly Spike Reading:**
```json
{
  "sensor_id": "temp_sensor_02",
  "timestamp": "2026-06-30T10:15:30.400000+00:00",
  "fields": {
    "temperature": 108.45
  }
}
```

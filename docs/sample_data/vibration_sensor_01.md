# Sensor: `vibration_sensor_01` (Dropout Simulator)

## Overview
This simulates a sensor that begins by functioning normally but suddenly stops sending data (network failure, power loss, etc.). It is used to test the Engine's heartbeat emission and Last Observation Carried Forward (LOCF) staleness flagging logic.

## Sensor Details
- **Sensor Name/ID**: `vibration_sensor_01`
- **Machine/Asset Type**: Factory Motor
- **Data Type**: Float
- **Firing Rate**: 10Hz (10 readings per second)
- **Normal Range**: 2.4 - 2.6 RMS (Base 2.5, Variance ±0.1)
- **Behavior**: Emits normally for the first 5 seconds, after which it goes completely silent indefinitely. The script stays alive to simulate a hanging connection or dead sensor without killing the process.

## Timestamping
Timestamps are in ISO-8601 UTC format (e.g., `2026-06-30T10:15:30.123456+00:00`).

## Sample JSON Structure

**Normal Reading (First 5 seconds):**
```json
{
  "sensor_id": "vibration_sensor_01",
  "timestamp": "2026-06-30T10:15:30.100000+00:00",
  "fields": {
    "vibration_rms": 2.456
  }
}
```

*After 5 seconds, no more JSON payloads are generated.*

# Sensor: `temp_sensor_01` (Steady Environment Sensor)

## Overview
This simulates a basic, highly stable environmental temperature sensor. It is used to test the Engine's Deadband Compression logic, as readings will frequently fall within the baseline variance and should be suppressed by the engine to save bandwidth.

## Sensor Details
- **Sensor Name/ID**: `temp_sensor_01`
- **Machine/Asset Type**: Ambient Environment Room / HVAC Unit
- **Data Type**: Float
- **Firing Rate**: 10Hz (10 readings per second)
- **Normal Range**: 89.5 - 90.5 °C (Base 90.0, Variance ±0.5)
- **Behavior**: Continuous, steady emission with minimal noise. No sudden spikes or dropouts.

## Timestamping
Timestamps are in ISO-8601 UTC format (e.g., `2026-06-30T10:15:30.123456+00:00`).

## Sample JSON Structure

```json
{
  "sensor_id": "temp_sensor_01",
  "timestamp": "2026-06-30T10:15:30.100000+00:00",
  "fields": {
    "temperature": 89.76
  }
}
```

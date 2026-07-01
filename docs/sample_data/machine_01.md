# Machine: `machine_01` (Multi-Rate Assembly Unit)

## Overview
This machine simulates a complex piece of equipment with multiple internal sensors firing at different, unaligned frequencies. It is designed to test the Adaptive ETL Engine's Stateful Latest-Value Join (Sensor Fusion) capabilities.

## Sensors

### 1. Temperature Sensor
- **Machine Name**: `machine_01`
- **Sensor Name**: `temperature`
- **Data Type**: Float
- **Firing Rate**: 10Hz (10 readings per second)
- **Normal Range**: 89.5 - 90.5 °C (Base 90.0, Variance ±0.5)
- **Behavior**: Emits continuously at a steady rate.

### 2. Pressure Sensor
- **Machine Name**: `machine_01`
- **Sensor Name**: `pressure`
- **Data Type**: Float
- **Firing Rate**: 15Hz (15 readings per second)
- **Normal Range**: 118.0 - 122.0 PSI (Base 120.0, Variance ±2.0)
- **Behavior**: Emits continuously at a steady rate.

## Timestamping
Timestamps are in ISO-8601 UTC format (e.g., `2026-06-30T10:15:30.123456+00:00`). Because the sensors fire at different frequencies (10Hz vs 15Hz), their timestamps rarely align perfectly.

## Sample JSON Structure

**Temperature Reading:**
```json
{
  "sensor_id": "machine_01",
  "timestamp": "2026-06-30T10:15:30.100000+00:00",
  "fields": {
    "temperature": 90.12
  }
}
```

**Pressure Reading:**
```json
{
  "sensor_id": "machine_01",
  "timestamp": "2026-06-30T10:15:30.166666+00:00",
  "fields": {
    "pressure": 121.45
  }
}
```

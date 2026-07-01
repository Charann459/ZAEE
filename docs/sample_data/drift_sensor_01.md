# Sensor: `drift_sensor_01` (Schema Drift Simulator)

## Overview
This simulates a sensor that begins by emitting a known schema (a single float field). After a set duration, the sensor undergoes a firmware update or structural change mid-stream, drastically altering its schema without notice. It is used to test the Engine's dynamic schema contradiction handling and isolation of new fields.

## Sensor Details
- **Sensor Name/ID**: `drift_sensor_01`
- **Machine/Asset Type**: Fluid Pipeline Flowmeter
- **Firing Rate**: 5Hz (5 readings per second)
- **Behavior**: Emits a single float field (`flow_rate`) normally for the first 10 seconds. After 10 seconds, it simultaneously changes the data type of `flow_rate` from Float to String, AND introduces a brand new integer field (`new_diagnostic_code`).

## Initial Schema
- **Field 1**: `flow_rate`
- **Data Type**: Float
- **Range**: 49.0 - 51.0 (Base 50.0, Variance ±1.0)

## Drifted Schema (After 10s)
- **Field 1**: `flow_rate`
- **Data Type**: String (e.g., "50.12")
- **Field 2 (NEW)**: `new_diagnostic_code`
- **Data Type**: Integer (Range 100 - 999)

## Timestamping
Timestamps are in ISO-8601 UTC format (e.g., `2026-06-30T10:15:30.123456+00:00`).

## Sample JSON Structure

**Initial Reading (First 10 seconds):**
```json
{
  "sensor_id": "drift_sensor_01",
  "timestamp": "2026-06-30T10:15:30.200000+00:00",
  "fields": {
    "flow_rate": 50.45
  }
}
```

**Drifted Reading (After 10 seconds):**
```json
{
  "sensor_id": "drift_sensor_01",
  "timestamp": "2026-06-30T10:15:40.200000+00:00",
  "fields": {
    "flow_rate": "49.88",
    "new_diagnostic_code": 742
  }
}
```

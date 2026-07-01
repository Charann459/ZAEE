# Machine-C: Hydraulic Test Rig (ZeMA)

## Overview
Machine-C represents a complex condition monitoring scenario on a hydraulic test rig provided by ZeMA gGmbH. Unlike Machine A and B, this dataset is a true multi-rate time-series dataset stored across multiple text files, making it an ideal candidate for testing the Adaptive ETL Engine's Sensor Fusion and timestamp alignment capabilities.

## Machine Details
- **Machine Name**: `Machine-C`
- **Machine Type**: Hydraulic System Test Rig
- **Behavior**: The rig experiences varying load cycles. Four fault types are superimposed with several severity grades (e.g., cooler condition, valve condition, internal pump leakage, hydraulic accumulator).

## Sensors, Types, and Firing Rates

The data is split by sensor type into separate text files, collected at three different sampling frequencies:

### 100 Hz Sensors (High Frequency)
- **PS1-PS6** (`PS1.txt` - `PS6.txt`): Pressure sensors (6 instances).
- **EPS1** (`EPS1.txt`): Motor power.

### 10 Hz Sensors (Medium Frequency)
- **FS1-FS2** (`FS1.txt`, `FS2.txt`): Volume flow sensors (2 instances).

### 1 Hz Sensors (Low Frequency)
- **TS1-TS4** (`TS1.txt` - `TS4.txt`): Temperature sensors (4 instances).
- **VS1** (`VS1.txt`): Vibration sensor.
- **CE** (`CE.txt`): Cooling efficiency (virtual/derived).
- **CP** (`CP.txt`): Cooling power (virtual/derived).
- **SE** (`SE.txt`): Efficiency factor (virtual/derived).

## Data Structure
- **Format**: Matrix text files. Each row represents a single 60-second measurement cycle.
- **Columns**: The number of columns depends on the sampling rate:
  - 1 Hz sensors have 60 columns (60 seconds * 1 Hz).
  - 10 Hz sensors have 600 columns (60 seconds * 10 Hz).
  - 100 Hz sensors have 6000 columns (60 seconds * 100 Hz).

## Target Profile
- `profile.txt`: Contains the target condition annotations (cooler condition, valve condition, internal pump leakage, hydraulic accumulator, stable flag) for each 60-second cycle.

## Timestamps & ETL Application
Because the data is explicitly recorded at 1Hz, 10Hz, and 100Hz, simulating this data requires parsing a row from each file simultaneously, expanding the columns into discrete time-series points, and emitting them at their respective frequencies. The Adaptive ETL Engine will then use its Redis-backed state cache to align and fuse these misaligned data streams back into a unified schema.

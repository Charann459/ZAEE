# Machine-B: Large Industrial Pump

## Overview
Machine-B represents a fleet of large industrial pumps. The dataset (`Large_Industrial_Pump_Maintenance_Dataset.csv`) is focused on predictive maintenance and condition monitoring across multiple distinct pump assets.

## Machine Details
- **Machine Name**: `Machine-B` (Multi-Asset)
- **Machine Type**: Industrial Pump
- **Behavior**: Pumps operate under varying RPMs and operational hours. The data tracks their state up to the point of required maintenance.

## Sensors & Attributes

- `Pump_ID` (Integer): Identifier for the specific pump (e.g., 2, 3, 4, 5).
- `Temperature` (Float): e.g., 61.8 - 127.5 °C. Higher temperatures often correlate with impending faults.
- `Vibration` (Float): e.g., 2.3 - 4.5. 
- `Pressure` (Float): e.g., 136.0 - 285.8 PSI.
- `Flow_Rate` (Float): e.g., 2.0 - 16.2.
- `RPM` (Float): Revolutions per minute of the pump motor (e.g., 1004.8 - 2597.6).
- `Operational_Hours` (Float): Total running time of the pump (e.g., 761.5 - 5489.0 hours).

### Target Label
- `Maintenance_Flag` (Integer: 0 or 1): Boolean flag indicating whether the pump requires maintenance (`1`) or is operating normally (`0`).

## Timestamps
Like Machine-A, this tabular dataset does not include explicit timestamps. It represents sequential observations or snapshots of pump state. To stream this data into the Adaptive ETL Engine, rows can be grouped by `Pump_ID` and emitted sequentially with injected synthetic timestamps to simulate live telemetry.

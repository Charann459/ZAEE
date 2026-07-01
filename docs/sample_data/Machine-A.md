# Machine-A: Industrial Fault Detection Dataset

## Overview
Machine-A represents a generic industrial machine with a dataset aimed at fault detection. The data is structured as a CSV file (`Industrial_fault_detection.csv`) and contains multivariate readings with pre-computed FFT (Fast Fourier Transform) features.

## Machine Details
- **Machine Name**: `Machine-A`
- **Machine Type**: Generic Industrial Equipment
- **Behavior**: Operates with continuous sensor readings. The data includes both standard operational states and fault states (indicated by `Fault_Type`).

## Sensors & Attributes
The dataset is highly denormalized and flattened into a single tabular format.

### 1. Primary Sensors
- `Temperature` (Float): e.g., 46.0 - 78.4 °C
- `Vibration` (Float): e.g., 2.0 - 3.3
- `Pressure` (Float): e.g., 56.7 - 101.9
- `Flow_Rate` (Float): e.g., 6.1 - 13.7
- `Current` (Float): e.g., 6.8 - 16.7
- `Voltage` (Float): e.g., 202.0 - 230.4

### 2. Derived Features (FFT)
For `Temperature`, `Vibration`, and `Pressure`, the dataset includes 10 separate FFT components (0 through 9) to capture frequency-domain characteristics.
- e.g., `FFT_Temp_0`, `FFT_Vib_0`, `FFT_Pres_0` ... up to `FFT_Temp_9`, `FFT_Vib_9`, `FFT_Pres_9`

### 3. Target Label
- `Fault_Type` (Integer): Categorical label indicating the health state of the machine (e.g., `0` for healthy, `3` for a specific fault).

## Timestamps
The dataset itself does not contain explicit timestamp columns. The rows are assumed to be sequential observations. For streaming simulation in the ETL engine, synthetic timestamps (e.g., generated at 1Hz or 10Hz) will need to be applied to each row as it is ingested.

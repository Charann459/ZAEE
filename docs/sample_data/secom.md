# SECOM: Semiconductor Manufacturing Dataset

## Overview
The SECOM dataset (from the UCI Machine Learning Repository) represents data from a complex modern semi-conductor manufacturing process. It is used to monitor signals and variables collected from sensors and process measurement points. The dataset is characterized by a high number of features (many of which may be irrelevant or noisy) and the presence of missing values.

## Machine/Process Details
- **Process Name**: `SECOM`
- **Process Type**: Semi-conductor Manufacturing
- **Behavior**: The process is under continuous surveillance. Each example (row) represents a single production entity (e.g., a silicon wafer or batch) with its associated measured features.

## Data Structure
The dataset is split into two primary files:

### 1. `secom.data` (Sensor Features)
- **Format**: Raw text file, space-separated.
- **Instances**: 1567 examples.
- **Features**: 591 real-valued attributes per example.
- **Missing Values**: Present and represented by the string `NaN`. 
- **Characteristics**: The 591 features represent various sensor readings and measurement points. The exact names of the sensors are anonymized, so they are typically treated as `Feature_1` through `Feature_591`.

### 2. `secom_labels.data` (Target Labels & Timestamps)
- **Format**: Raw text file, space-separated.
- **Columns**: 2 columns per row.
  1. **Label**: `-1` corresponds to a "Pass" (normal yield), and `1` corresponds to a "Fail" (yield excursion / fault). Out of 1567 examples, there are 104 fails.
  2. **Timestamp**: Date and time of the specific test point (e.g., `"19/07/2008 11:55:00"`).

## Timestamps & ETL Application
Unlike Machine A and B, the SECOM dataset provides explicit, real-world timestamps for every row in the `secom_labels.data` file. 

To stream this data into the Adaptive ETL Engine:
1. The rows from `secom.data` and `secom_labels.data` must be zipped together line-by-line.
2. The engine will need to handle the wide schema (591 fields) and cleanly process or impute the `NaN` values.
3. Feature selection or deadband filtering in the engine will be highly relevant here to strip out the constant or purely noisy features among the 591 inputs before passing the payload downstream.

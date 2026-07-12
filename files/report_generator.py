#!/usr/bin/env python3
"""
ZAEE Test Evidence Report Generator
=====================================
Reads:
  - Go test JSON output  (go test -v -json ./... > test_results.json)
  - System metrics JSON  (produced by system_test.go → saveMetrics())

Produces:
  - report.html  — human-readable evidence report for capstone review

Usage:
  go test -v -json ./tests/unit/... > test_results.json
  go test -v -json -tags=integration ./tests/integration/... >> test_results.json
  python3 tests/report_generator.py --test-results test_results.json --metrics test_metrics_*.json
"""

import argparse
import json
import sys
import glob
from datetime import datetime
from pathlib import Path


def load_test_results(filepath: str) -> list[dict]:
    """Parse Go's -json test output. Each line is a JSON object."""
    results = []
    try:
        with open(filepath) as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    results.append(json.loads(line))
                except json.JSONDecodeError:
                    continue
    except FileNotFoundError:
        print(f"Warning: test results file not found: {filepath}", file=sys.stderr)
    return results


def load_metrics(pattern: str) -> list[dict]:
    """Load all system metrics JSON files matching the glob pattern."""
    metrics = []
    for filepath in glob.glob(pattern):
        try:
            with open(filepath) as f:
                metrics.append(json.load(f))
        except (FileNotFoundError, json.JSONDecodeError) as e:
            print(f"Warning: could not load metrics from {filepath}: {e}", file=sys.stderr)
    return metrics


def summarize_tests(raw_results: list[dict]) -> dict:
    """Aggregate Go test output into pass/fail/skip counts per package."""
    summary = {
        "total": 0,
        "passed": 0,
        "failed": 0,
        "skipped": 0,
        "packages": {},
        "failures": [],
    }

    for event in raw_results:
        action = event.get("Action", "")
        test = event.get("Test", "")
        package = event.get("Package", "")
        elapsed = event.get("Elapsed", 0)
        output = event.get("Output", "")

        if not test:
            continue  # package-level event, skip

        pkg_data = summary["packages"].setdefault(package, {
            "passed": 0, "failed": 0, "skipped": 0, "tests": []
        })

        if action == "pass":
            summary["total"] += 1
            summary["passed"] += 1
            pkg_data["passed"] += 1
            pkg_data["tests"].append({
                "name": test, "status": "PASS", "elapsed": elapsed
            })

        elif action == "fail":
            summary["total"] += 1
            summary["failed"] += 1
            pkg_data["failed"] += 1
            pkg_data["tests"].append({
                "name": test, "status": "FAIL", "elapsed": elapsed
            })
            summary["failures"].append({
                "package": package, "test": test, "output": output
            })

        elif action == "skip":
            summary["total"] += 1
            summary["skipped"] += 1
            pkg_data["skipped"] += 1
            pkg_data["tests"].append({
                "name": test, "status": "SKIP", "elapsed": elapsed
            })

    return summary


def status_badge(status: str) -> str:
    colors = {"PASS": "#2ea44f", "FAIL": "#d73a49", "SKIP": "#e36209"}
    color = colors.get(status, "#586069")
    return f'<span style="background:{color};color:white;padding:2px 8px;border-radius:3px;font-size:12px;font-weight:bold">{status}</span>'


def generate_html(test_summary: dict, all_metrics: list[dict]) -> str:
    now = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    pass_rate = (test_summary["passed"] / test_summary["total"] * 100) if test_summary["total"] > 0 else 0
    overall_status = "PASS" if test_summary["failed"] == 0 else "FAIL"
    overall_color = "#2ea44f" if overall_status == "PASS" else "#d73a49"

    # Build package rows
    package_rows = ""
    for pkg, data in test_summary["packages"].items():
        pkg_short = pkg.split("/")[-1] if "/" in pkg else pkg
        pkg_status = "PASS" if data["failed"] == 0 else "FAIL"
        package_rows += f"""
        <tr>
            <td><code>{pkg_short}</code></td>
            <td>{status_badge(pkg_status)}</td>
            <td style="color:#2ea44f">{data['passed']}</td>
            <td style="color:#d73a49">{data['failed']}</td>
            <td style="color:#e36209">{data['skipped']}</td>
        </tr>"""

    # Build individual test rows
    test_rows = ""
    for pkg, data in test_summary["packages"].items():
        pkg_short = pkg.split("/")[-1] if "/" in pkg else pkg
        for test in data["tests"]:
            test_rows += f"""
        <tr>
            <td><code>{pkg_short}</code></td>
            <td>{test['name']}</td>
            <td>{status_badge(test['status'])}</td>
            <td>{test['elapsed']:.3f}s</td>
        </tr>"""

    # Build system metrics section
    metrics_section = ""
    for m in all_metrics:
        reduction = m.get("reduction_pct", 0)
        reduction_color = "#2ea44f" if reduction >= 50 else "#d73a49"
        metrics_section += f"""
        <div class="metrics-card">
            <h3>System Run: <code>{m.get('run_id', 'unknown')}</code></h3>
            <div class="metrics-grid">
                <div class="metric">
                    <div class="metric-value">{m.get('total_machines', 0)}</div>
                    <div class="metric-label">Machines</div>
                </div>
                <div class="metric">
                    <div class="metric-value">{m.get('total_sensors', 0)}</div>
                    <div class="metric-label">Sensors</div>
                </div>
                <div class="metric">
                    <div class="metric-value">{m.get('total_messages_ingested', 0):,}</div>
                    <div class="metric-label">Messages Ingested</div>
                </div>
                <div class="metric">
                    <div class="metric-value">{m.get('total_messages_emitted', 0):,}</div>
                    <div class="metric-label">Messages Emitted</div>
                </div>
                <div class="metric">
                    <div class="metric-value" style="color:{reduction_color}">{reduction:.1f}%</div>
                    <div class="metric-label">Data Reduction</div>
                </div>
                <div class="metric">
                    <div class="metric-value">{m.get('locf_fill_count', 0):,}</div>
                    <div class="metric-label">LOCF Gap Fills</div>
                </div>
                <div class="metric">
                    <div class="metric-value">{m.get('flag_count', 0):,}</div>
                    <div class="metric-label">Flags Emitted</div>
                </div>
                <div class="metric">
                    <div class="metric-value">{m.get('sdt_corridor_breaks', 0):,}</div>
                    <div class="metric-label">SDT Corridor Breaks</div>
                </div>
            </div>
            <p>Duration: <strong>{m.get('duration_seconds', 0):.0f}s</strong> &nbsp;|&nbsp;
               Test Passed: {status_badge('PASS' if m.get('test_passed') else 'FAIL')}</p>
        </div>"""

    if not metrics_section:
        metrics_section = "<p style='color:#586069'>No system metrics files found. Run system tests to generate.</p>"

    # Build failure details
    failure_section = ""
    if test_summary["failures"]:
        failure_section = "<h2>Failure Details</h2>"
        for f in test_summary["failures"]:
            failure_section += f"""
        <div style="background:#fff5f5;border:1px solid #d73a49;border-radius:6px;padding:16px;margin-bottom:12px">
            <strong>{f['test']}</strong> in <code>{f['package']}</code>
            <pre style="margin-top:8px;font-size:12px;overflow-x:auto">{f['output']}</pre>
        </div>"""

    return f"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>ZAEE Test Evidence Report</title>
<style>
  body {{ font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; max-width: 1100px; margin: 0 auto; padding: 32px; color: #24292e; }}
  h1 {{ border-bottom: 2px solid #e1e4e8; padding-bottom: 12px; }}
  h2 {{ color: #24292e; margin-top: 32px; }}
  h3 {{ color: #586069; }}
  .summary-bar {{ display: flex; gap: 24px; padding: 20px; background: #f6f8fa; border-radius: 8px; margin: 24px 0; }}
  .summary-item {{ text-align: center; }}
  .summary-value {{ font-size: 32px; font-weight: bold; }}
  .summary-label {{ font-size: 13px; color: #586069; }}
  table {{ width: 100%; border-collapse: collapse; margin: 16px 0; }}
  th {{ background: #f6f8fa; padding: 10px 12px; text-align: left; border-bottom: 2px solid #e1e4e8; font-size: 13px; }}
  td {{ padding: 8px 12px; border-bottom: 1px solid #e1e4e8; font-size: 13px; }}
  tr:hover {{ background: #f6f8fa; }}
  .overall {{ display: inline-block; padding: 8px 20px; border-radius: 6px; color: white; font-weight: bold; font-size: 18px; background: {overall_color}; }}
  .metrics-card {{ background: #f6f8fa; border: 1px solid #e1e4e8; border-radius: 8px; padding: 20px; margin-bottom: 16px; }}
  .metrics-grid {{ display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 16px; margin: 16px 0; }}
  .metric {{ text-align: center; background: white; padding: 12px; border-radius: 6px; border: 1px solid #e1e4e8; }}
  .metric-value {{ font-size: 22px; font-weight: bold; color: #0366d6; }}
  .metric-label {{ font-size: 11px; color: #586069; margin-top: 4px; }}
  code {{ background: #f3f4f6; padding: 2px 6px; border-radius: 3px; font-size: 12px; }}
  pre {{ background: #f6f8fa; padding: 12px; border-radius: 6px; overflow-x: auto; }}
  .footer {{ margin-top: 48px; padding-top: 16px; border-top: 1px solid #e1e4e8; color: #586069; font-size: 12px; }}
</style>
</head>
<body>
<h1>ZAEE — Adaptive ETL Engine<br><small style="color:#586069;font-size:16px">Test Evidence Report</small></h1>

<p>Generated: <strong>{now}</strong></p>

<h2>Overall Result</h2>
<div class="summary-bar">
  <div class="summary-item">
    <div class="overall">{overall_status}</div>
  </div>
  <div class="summary-item">
    <div class="summary-value">{test_summary['total']}</div>
    <div class="summary-label">Total Tests</div>
  </div>
  <div class="summary-item">
    <div class="summary-value" style="color:#2ea44f">{test_summary['passed']}</div>
    <div class="summary-label">Passed</div>
  </div>
  <div class="summary-item">
    <div class="summary-value" style="color:#d73a49">{test_summary['failed']}</div>
    <div class="summary-label">Failed</div>
  </div>
  <div class="summary-item">
    <div class="summary-value" style="color:#e36209">{test_summary['skipped']}</div>
    <div class="summary-label">Skipped</div>
  </div>
  <div class="summary-item">
    <div class="summary-value" style="color:#0366d6">{pass_rate:.1f}%</div>
    <div class="summary-label">Pass Rate</div>
  </div>
</div>

<h2>Results by Package</h2>
<table>
  <thead><tr><th>Package</th><th>Status</th><th>Passed</th><th>Failed</th><th>Skipped</th></tr></thead>
  <tbody>{package_rows}</tbody>
</table>

<h2>Individual Test Results</h2>
<table>
  <thead><tr><th>Package</th><th>Test</th><th>Status</th><th>Duration</th></tr></thead>
  <tbody>{test_rows}</tbody>
</table>

{failure_section}

<h2>System Performance Metrics</h2>
{metrics_section}

<div class="footer">
  <p>ZAEE — Zero-Assumption Edge ETL Engine | Capstone Project</p>
  <p>Report generated by <code>tests/report_generator.py</code></p>
</div>
</body>
</html>"""


def main():
    parser = argparse.ArgumentParser(description="Generate ZAEE test evidence report")
    parser.add_argument("--test-results", default="test_results.json",
                        help="Path to Go test JSON output file")
    parser.add_argument("--metrics", default="test_metrics_*.json",
                        help="Glob pattern for system metrics JSON files")
    parser.add_argument("--output", default="report.html",
                        help="Output HTML report path")
    args = parser.parse_args()

    print(f"Loading test results from: {args.test_results}")
    raw_results = load_test_results(args.test_results)

    print(f"Loading system metrics from: {args.metrics}")
    all_metrics = load_metrics(args.metrics)

    print("Summarising test results...")
    summary = summarize_tests(raw_results)

    print(f"  Total: {summary['total']} | Passed: {summary['passed']} | Failed: {summary['failed']} | Skipped: {summary['skipped']}")

    print("Generating HTML report...")
    html = generate_html(summary, all_metrics)

    output_path = Path(args.output)
    output_path.write_text(html, encoding="utf-8")
    print(f"Report saved to: {output_path.resolve()}")

    # Exit with non-zero if any tests failed — useful in CI
    if summary["failed"] > 0:
        print(f"\n{summary['failed']} test(s) failed.", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()

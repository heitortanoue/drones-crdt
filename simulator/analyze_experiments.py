#!/usr/bin/env python3
"""
Analyze metrics collected from experiments.
Generates summary statistics and comparison tables.
"""

import json
import sys
from pathlib import Path
from typing import Dict, List

import pandas as pd


def load_experiment_metrics(experiment_dir: Path) -> pd.DataFrame:
    """Load metrics.csv from an experiment directory."""
    metrics_file = experiment_dir / "metrics.csv"
    if not metrics_file.exists():
        print(f"Warning: No metrics.csv found in {experiment_dir}")
        return None

    df = pd.read_csv(metrics_file)
    return df


def analyze_single_experiment(experiment_dir: Path) -> Dict:
    """Analyze a single experiment and return summary stats."""
    # Load metrics
    df = load_experiment_metrics(experiment_dir)
    if df is None or df.empty:
        return None

    # Load experiment config
    config_file = experiment_dir / "experiment.json"
    with open(config_file, "r") as f:
        config = json.load(f)

    # Calculate summary statistics
    summary = {
        "experiment_id": config["id"],
        "description": config["description"],
        "parameters": config["parameters"],
        "metrics": {
            # Network load
            "avg_msgs_sent_per_sec": (
                df.groupby("drone_id")["msgs_sent_total"]
                .apply(lambda x: x.diff().mean())
                .mean()
                / config["parameters"]["sample_interval_sec"]
            ),
            "avg_bytes_sent_per_sec": (
                df.groupby("drone_id")["bytes_sent_total"]
                .apply(lambda x: x.diff().mean())
                .mean()
                / config["parameters"]["sample_interval_sec"]
            ),
            # Duplication
            "avg_duplicates_dropped": df.groupby("drone_id")["duplicates_dropped"]
            .max()
            .mean(),
            "avg_dedup_cache_size": df["dedup_cache_size"].mean(),
            # State
            "avg_active_elements": df["active_elements"].mean(),
            "max_active_elements": df["active_elements"].max(),
            "final_active_elements": df.groupby("drone_id")["active_elements"]
            .last()
            .mean(),
            # Neighbors
            "avg_neighbor_count": df["neighbor_count"].mean(),
            # Dissemination
            "total_delta_messages": df.groupby("drone_id")["delta_messages_sent"]
            .max()
            .sum(),
            "total_ae_messages": df.groupby("drone_id")["anti_entropy_messages_sent"]
            .max()
            .sum(),
            "ae_to_delta_ratio": (
                df.groupby("drone_id")["anti_entropy_messages_sent"].max().sum()
                / max(1, df.groupby("drone_id")["delta_messages_sent"].max().sum())
            ),
        },
    }

    return summary


def compare_experiments(results_dir: Path = Path("experiment_results")):
    """Compare all experiments and generate comparison table."""
    if not results_dir.exists():
        print(f"Error: Results directory {results_dir} not found")
        return

    # Find all experiment directories
    experiment_dirs = []
    for exp_dir in results_dir.iterdir():
        if exp_dir.is_dir():
            # Look for timestamped subdirectories
            for run_dir in exp_dir.iterdir():
                if run_dir.is_dir() and (run_dir / "metrics.csv").exists():
                    experiment_dirs.append(run_dir)

    if not experiment_dirs:
        print("No experiment results found")
        return

    print(f"\n{'='*100}")
    print(f"EXPERIMENT COMPARISON - {len(experiment_dirs)} experiments found")
    print(f"{'='*100}\n")

    # Analyze each experiment
    summaries = []
    for exp_dir in sorted(experiment_dirs):
        print(f"Analyzing {exp_dir.parent.name}/{exp_dir.name}...")
        summary = analyze_single_experiment(exp_dir)
        if summary:
            summaries.append(summary)

    if not summaries:
        print("No valid experiment data found")
        return

    # Create comparison table
    print(f"\n{'-'*100}")
    print("NETWORK LOAD COMPARISON")
    print(f"{'-'*100}")
    print(
        f"{'Experiment ID':<30} {'N':>5} {'F':>3} {'TTL':>4} {'Msgs/s':>10} {'Bytes/s':>12} {'Dups':>8}"
    )
    print(f"{'-'*100}")

    for s in summaries:
        params = s["parameters"]
        metrics = s["metrics"]
        print(
            f"{s['experiment_id']:<30} "
            f"{params['drone_count']:>5} "
            f"{params['fanout']:>3} "
            f"{params['ttl']:>4} "
            f"{metrics['avg_msgs_sent_per_sec']:>10.2f} "
            f"{metrics['avg_bytes_sent_per_sec']:>12.0f} "
            f"{metrics['avg_duplicates_dropped']:>8.1f}"
        )

    print(f"\n{'-'*100}")
    print("STATE & CONVERGENCE COMPARISON")
    print(f"{'-'*100}")
    print(
        f"{'Experiment ID':<30} {'Avg Active':>12} {'Max Active':>12} {'Final Avg':>12} {'Neighbors':>10}"
    )
    print(f"{'-'*100}")

    for s in summaries:
        metrics = s["metrics"]
        print(
            f"{s['experiment_id']:<30} "
            f"{metrics['avg_active_elements']:>12.1f} "
            f"{metrics['max_active_elements']:>12.0f} "
            f"{metrics['final_active_elements']:>12.1f} "
            f"{metrics['avg_neighbor_count']:>10.1f}"
        )

    print(f"\n{'-'*100}")
    print("DISSEMINATION COMPARISON")
    print(f"{'-'*100}")
    print(
        f"{'Experiment ID':<30} {'Total DELTA':>12} {'Total AE':>12} {'AE/DELTA':>10}"
    )
    print(f"{'-'*100}")

    for s in summaries:
        metrics = s["metrics"]
        print(
            f"{s['experiment_id']:<30} "
            f"{metrics['total_delta_messages']:>12.0f} "
            f"{metrics['total_ae_messages']:>12.0f} "
            f"{metrics['ae_to_delta_ratio']:>10.3f}"
        )

    print(f"\n{'='*100}\n")

    # Save to JSON
    output_file = results_dir / "comparison_summary.json"
    with open(output_file, "w") as f:
        json.dump(summaries, f, indent=2)
    print(f"Detailed comparison saved to: {output_file}\n")


def plot_experiment_timeseries(experiment_dir: Path, metric: str = "active_elements"):
    """Plot a metric over time for an experiment."""
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("matplotlib not installed. Install with: pip install matplotlib")
        return

    df = load_experiment_metrics(experiment_dir)
    if df is None or df.empty:
        return

    # Plot each drone's metric over time
    plt.figure(figsize=(12, 6))

    for drone_id in df["drone_id"].unique():
        drone_df = df[df["drone_id"] == drone_id]
        # Convert timestamp to relative time
        relative_time = drone_df["t"] - drone_df["t"].min()
        plt.plot(relative_time, drone_df[metric], label=drone_id, alpha=0.7)

    plt.xlabel("Time (seconds)")
    plt.ylabel(metric.replace("_", " ").title())
    plt.title(f"{metric} Over Time - {experiment_dir.parent.name}")
    # plt.legend(bbox_to_anchor=(1.05, 1), loc="upper left")
    plt.grid(True, alpha=0.3)
    plt.tight_layout()

    # Save plot
    output_file = experiment_dir / f"{metric}_timeseries.png"
    plt.savefig(output_file, dpi=150)
    print(f"Plot saved to: {output_file}")
    plt.close()


def plot_drone_positions(experiment_dir: Path):
    """Plot drone positions over time (trajectory visualization)."""
    try:
        import matplotlib.pyplot as plt
        import numpy as np
    except ImportError:
        print("matplotlib not installed. Install with: pip install matplotlib")
        return

    df = load_experiment_metrics(experiment_dir)
    if df is None or df.empty:
        return

    # Check if position columns exist
    if "pos_x" not in df.columns or "pos_y" not in df.columns:
        print("Warning: Position data not found in metrics")
        return

    # Create figure with subplots
    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(16, 7))

    # Plot 1: Trajectories
    for drone_id in df["drone_id"].unique():
        drone_df = df[df["drone_id"] == drone_id].sort_values("t")
        ax1.plot(
            drone_df["pos_x"],
            drone_df["pos_y"],
            marker="o",
            markersize=2,
            alpha=0.6,
            label=drone_id,
        )
        # Mark start and end
        ax1.plot(
            drone_df["pos_x"].iloc[0],
            drone_df["pos_y"].iloc[0],
            "go",
            markersize=8,
            alpha=0.8,
        )  # Start
        ax1.plot(
            drone_df["pos_x"].iloc[-1],
            drone_df["pos_y"].iloc[-1],
            "r^",
            markersize=8,
            alpha=0.8,
        )  # End

    ax1.set_xlabel("X Position")
    ax1.set_ylabel("Y Position")
    ax1.set_title(f"Drone Trajectories - {experiment_dir.parent.name}")
    # ax1.legend(bbox_to_anchor=(1.05, 1), loc="upper left", fontsize=8)
    ax1.grid(True, alpha=0.3)
    ax1.set_aspect("equal")

    # Plot 2: Position over time (both X and Y)
    relative_time = df["t"] - df["t"].min()
    for drone_id in df["drone_id"].unique():
        drone_df = df[df["drone_id"] == drone_id]
        drone_time = drone_df["t"] - df["t"].min()
        ax2.plot(drone_time, drone_df["pos_x"], alpha=0.5, linestyle="-")
        ax2.plot(drone_time, drone_df["pos_y"], alpha=0.5, linestyle="--")

    ax2.set_xlabel("Time (seconds)")
    ax2.set_ylabel("Position")
    ax2.set_title("Position vs Time (solid=X, dashed=Y)")
    ax2.grid(True, alpha=0.3)

    plt.tight_layout()

    # Save plot
    output_file = experiment_dir / "position_trajectory.png"
    plt.savefig(output_file, dpi=150)
    print(f"Position plot saved to: {output_file}")
    plt.close()


def main():
    """Main entry point."""
    if len(sys.argv) > 1:
        if sys.argv[1] == "plot":
            if len(sys.argv) < 3:
                print(
                    "Usage: python analyze_experiments.py plot <experiment_dir> [metric]"
                )
                print(
                    "       python analyze_experiments.py plot_position <experiment_dir>"
                )
                return
            experiment_dir = Path(sys.argv[2])
            metric = sys.argv[3] if len(sys.argv) > 3 else "active_elements"
            plot_experiment_timeseries(experiment_dir, metric)
        elif sys.argv[1] == "plot_position":
            if len(sys.argv) < 3:
                print(
                    "Usage: python analyze_experiments.py plot_position <experiment_dir>"
                )
                return
            experiment_dir = Path(sys.argv[2])
            plot_drone_positions(experiment_dir)
        else:
            results_dir = Path(sys.argv[1])
            compare_experiments(results_dir)
    else:
        compare_experiments()


if __name__ == "__main__":
    main()

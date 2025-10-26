"""
Metrics collection for drone simulation - no Go code changes required.
Collects data from Mininet-WiFi + existing HTTP endpoints.
"""

import csv
import json
import time
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Set

from config import DRONE_RANGE, FETCH_INTERVAL, TCP_PORT
from drone_utils import fetch_state, fetch_stats


class MetricsCollector:
    def __init__(self, output_dir: str, scenario_id: str):
        self.output_dir = Path(output_dir)
        self.output_dir.mkdir(parents=True, exist_ok=True)
        self.scenario_id = scenario_id

        # CSV writers for different metric types
        self.writers = {}
        self.files = {}

        # Initialize metric files
        self._init_network_metrics()
        self._init_crdt_metrics()
        self._init_topology_metrics()
        self._init_convergence_metrics()

    def _init_network_metrics(self):
        """Network load and traffic metrics."""
        path = self.output_dir / "network_load.csv"
        f = open(path, "w", newline="")
        writer = csv.writer(f)
        writer.writerow(
            [
                "timestamp",
                "drone_id",
                "scenario_id",
                "bytes_sent_total",
                "bytes_recv_total",
                "msgs_sent_total",
                "msgs_recv_total",
                "delta_msgs_sent",
                "ae_msgs_sent",
                "hello_msgs_sent",
                "duplicates_dropped",
                "dedup_cache_size",
            ]
        )
        self.files["network"] = f
        self.writers["network"] = writer

    def _init_crdt_metrics(self):
        """CRDT state and overhead metrics."""
        path = self.output_dir / "crdt_state.csv"
        f = open(path, "w", newline="")
        writer = csv.writer(f)
        writer.writerow(
            [
                "timestamp",
                "drone_id",
                "scenario_id",
                "active_elements",
                "dotcloud_size",
                "clock_entries",
                "state_entries",
                "all_deltas_json",
            ]
        )
        self.files["crdt"] = f
        self.writers["crdt"] = writer

    def _init_topology_metrics(self):
        """Topology and neighbor discovery metrics."""
        path = self.output_dir / "topology.csv"
        f = open(path, "w", newline="")
        writer = csv.writer(f)
        writer.writerow(
            [
                "timestamp",
                "drone_id",
                "scenario_id",
                "pos_x",
                "pos_y",
                "pos_z",
                "ground_truth_neighbors",
                "actual_neighbors",
                "precision",
                "recall",
                "f1_score",
            ]
        )
        self.files["topology"] = f
        self.writers["topology"] = writer

    def _init_convergence_metrics(self):
        """Convergence tracking."""
        path = self.output_dir / "convergence.csv"
        f = open(path, "w", newline="")
        writer = csv.writer(f)
        writer.writerow(
            [
                "timestamp",
                "scenario_id",
                "iteration",
                "convergence_index",
                "num_drones",
                "avg_delta_count",
                "min_delta_count",
                "max_delta_count",
            ]
        )
        self.files["convergence"] = f
        self.writers["convergence"] = writer

    def collect_network_metrics(self, drone, timestamp: float):
        """Collect network load from /stats endpoint."""
        stats = fetch_stats(drone)
        if not stats:
            return

        self.writers["network"].writerow(
            [
                timestamp,
                drone.name,
                self.scenario_id,
                stats.get("bytes_sent", 0),
                stats.get("bytes_received", 0),
                stats.get("messages_sent", 0),
                stats.get("messages_received", 0),
                stats.get("delta_messages_sent", 0),
                stats.get("anti_entropy_messages_sent", 0),
                stats.get("hello_messages_sent", 0),
                stats.get("duplicates_dropped", 0),
                stats.get("cache_size", 0),
            ]
        )

    def collect_crdt_metrics(self, drone, timestamp: float):
        """Collect CRDT state from /state endpoint."""
        _, confidence, all_deltas = fetch_state(drone)
        if all_deltas is None:
            return

        self.writers["crdt"].writerow(
            [
                timestamp,
                drone.name,
                self.scenario_id,
                len(all_deltas),  # active elements
                0,  # dotcloud_size - would need /stats extension
                0,  # clock_entries - would need /stats extension
                len(all_deltas),  # state_entries (same for now)
                json.dumps(all_deltas),
            ]
        )

    def collect_topology_metrics(self, drone, drones: list, timestamp: float):
        """Collect position and neighbor discovery metrics."""
        # Ground truth: calculate which drones SHOULD be neighbors
        pos = drone.position
        ground_truth = set()

        for other in drones:
            if other.name == drone.name:
                continue
            other_pos = other.position
            dist = ((pos[0] - other_pos[0]) ** 2 + (pos[1] - other_pos[1]) ** 2) ** 0.5
            if dist <= DRONE_RANGE:
                ground_truth.add(other.name)

        # Actual neighbors from the drone (if /neighbors endpoint exists)
        # For now, we estimate from /stats or assume not available
        actual_neighbors = set()  # Would need /neighbors endpoint

        # Calculate precision/recall
        if len(actual_neighbors) > 0:
            intersection = ground_truth & actual_neighbors
            precision = len(intersection) / len(actual_neighbors)
            recall = len(intersection) / len(ground_truth) if ground_truth else 1.0
            f1 = (
                2 * (precision * recall) / (precision + recall)
                if (precision + recall) > 0
                else 0
            )
        else:
            precision = recall = f1 = 0.0

        self.writers["topology"].writerow(
            [
                timestamp,
                drone.name,
                self.scenario_id,
                pos[0],
                pos[1],
                pos[2],
                json.dumps(sorted(ground_truth)),
                json.dumps(sorted(actual_neighbors)),
                precision,
                recall,
                f1,
            ]
        )

    def collect_convergence_metrics(
        self,
        drones: list,
        timestamp: float,
        iteration: int,
        convergence_idx: float,
        drone_delta_sets: List[Set[str]],
    ):
        """Collect convergence metrics."""
        delta_counts = [len(s) for s in drone_delta_sets]

        self.writers["convergence"].writerow(
            [
                timestamp,
                self.scenario_id,
                iteration,
                convergence_idx,
                len(drones),
                sum(delta_counts) / len(delta_counts) if delta_counts else 0,
                min(delta_counts) if delta_counts else 0,
                max(delta_counts) if delta_counts else 0,
            ]
        )

    def close(self):
        """Close all open files."""
        for f in self.files.values():
            f.close()


def collect_metrics_loop(drones, stop_event, scenario_id: str, output_dir: str):
    """
    Main collection loop - replaces/extends fetch_states().
    Collects all available metrics without modifying Go code.
    """
    collector = MetricsCollector(output_dir, scenario_id)
    iteration = 0

    try:
        while not stop_event.is_set():
            stop_event.wait(FETCH_INTERVAL)
            if stop_event.is_set():
                break

            timestamp = time.time()
            drone_delta_sets: List[Set[str]] = [set() for _ in drones]

            # Collect per-drone metrics
            for i, drone in enumerate(drones):
                # Network load
                collector.collect_network_metrics(drone, timestamp)

                # CRDT state
                _, _, all_deltas = fetch_state(drone)
                if all_deltas:
                    for fire_with_meta in all_deltas:
                        cell = fire_with_meta["cell"]
                        cell_key = f"{cell['x']},{cell['y']}"
                        drone_delta_sets[i].add(cell_key)

                    collector.collect_crdt_metrics(drone, timestamp)

                # Topology
                collector.collect_topology_metrics(drone, drones, timestamp)

            # Calculate convergence (from your existing code)
            from drone_utils import convergence_index

            convergence = convergence_index(drone_delta_sets)
            collector.collect_convergence_metrics(
                drones, timestamp, iteration, convergence, drone_delta_sets
            )

            iteration += 1
            print(
                f"[{datetime.fromtimestamp(timestamp).isoformat()}] "
                f"Iteration {iteration}: Convergence = {convergence:.4f}"
            )

            if convergence == 1.0:
                print(f"✓ Convergence achieved after {iteration * FETCH_INTERVAL}s")

    finally:
        collector.close()


def save_scenario_metadata(output_dir: str, scenario_config: dict):
    """Save scenario configuration for reproducibility."""
    path = Path(output_dir) / "scenario.json"
    with open(path, "w") as f:
        json.dump(scenario_config, f, indent=2)

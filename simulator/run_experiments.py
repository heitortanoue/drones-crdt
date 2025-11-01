#!/usr/bin/env python3
"""
Automated experiment runner for drone CRDT evaluation.
Runs experiments defined in experiments.json and collects comprehensive metrics.
"""

import csv
import json
import os
import random
import threading
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime
from pathlib import Path
from typing import Dict, List

from config import BIND_ADDR, EXEC_PATH, SIMULATION_MULTIPLIER, TCP_PORT, UDP_PORT
from drone_utils import send_locations, setup_topology
from mininet.log import info, setLogLevel
from traffic_analyzer import TrafficAnalyzer


class ExperimentRunner:
    """Manages experiment execution and data collection."""

    def __init__(self, experiments_file: str = "experiments.json"):
        self.experiments_file = experiments_file
        self.experiments = self.load_experiments()
        self.base_output_dir = Path("experiment_results")
        self.base_output_dir.mkdir(exist_ok=True)

    def load_experiments(self) -> List[Dict]:
        """Load experiment configurations from JSON file."""
        with open(self.experiments_file, "r") as f:
            data = json.load(f)
        return [exp for exp in data["experiments"] if exp.get("enabled", True)]

    def run_all_experiments(self):
        """Run all enabled experiments sequentially."""
        info(f"\n{'='*80}\n")
        info(f"EXPERIMENT SUITE: {len(self.experiments)} experiments to run\n")
        info(f"{'='*80}\n\n")

        for idx, experiment in enumerate(self.experiments, 1):
            info(f"\n{'='*80}\n")
            info(f"Experiment {idx}/{len(self.experiments)}: {experiment['id']}\n")
            info(f"{experiment['description']}\n")
            info(f"{'='*80}\n\n")

            try:
                self.run_single_experiment(experiment)
                info(f"\n✓ Experiment {experiment['id']} completed successfully\n")
            except Exception as e:
                info(f"\n✗ Experiment {experiment['id']} failed: {e}\n")
                continue

            # Cooldown between experiments
            if idx < len(self.experiments):
                info("\n--- Cooldown period (30 seconds) ---\n")
                time.sleep(30)

        info(f"\n{'='*80}\n")
        info(f"EXPERIMENT SUITE COMPLETE\n")
        info(f"Results in: {self.base_output_dir}\n")
        info(f"{'='*80}\n\n")

    def run_single_experiment(self, experiment: Dict):
        """Run a single experiment configuration."""
        # Clean up any existing Mininet processes first
        self._cleanup_mininet()

        # Extract parameters
        params = experiment["parameters"]
        scenario_id = experiment["id"]

        # Create output directory
        output_dir = (
            self.base_output_dir
            / scenario_id
            / datetime.now().strftime("%Y%m%d_%H%M%S")
        )
        output_dir.mkdir(parents=True, exist_ok=True)

        # Save experiment metadata
        with open(output_dir / "experiment.json", "w") as f:
            json.dump(experiment, f, indent=2)

        # Setup topology
        info("*** Creating network topology ***\n")
        net, drones = self._setup_topology(params)

        # Apply network conditions (loss, etc.)
        if params.get("loss_rate_percent", 0) > 0:
            self._apply_network_loss(drones, params["loss_rate_percent"])

        # Start traffic capture
        info("*** Starting traffic capture ***\n")
        traffic_analyzer = TrafficAnalyzer(str(output_dir / "traffic"))
        for drone in drones:
            traffic_analyzer.start_capture(drone, tcp_port=TCP_PORT, udp_port=UDP_PORT)

        # Start drone applications
        info("*** Starting drone applications ***\n")
        self._start_drone_apps(drones, params)

        # Wait for initialization
        time.sleep(10)

        # Start data collection
        info("*** Starting metrics collection ***\n")
        stop_event = threading.Event()
        collector = MetricsCollector(
            str(output_dir), scenario_id, params["sample_interval_sec"]
        )

        collection_thread = threading.Thread(
            target=self._collect_metrics_loop,
            args=(drones, stop_event, collector, params["sample_interval_sec"]),
            daemon=True,
        )
        collection_thread.start()

        # Start position updates
        location_thread = threading.Thread(
            target=send_locations, args=(drones, stop_event), daemon=True
        )
        location_thread.start()

        # Run for specified duration
        duration = params["duration_sec"]
        info(f"*** Running experiment for {duration} seconds ***\n")

        try:
            time.sleep(duration)
        except KeyboardInterrupt:
            info("\n*** Experiment interrupted by user ***\n")

        # Stop data collection
        info("*** Stopping data collection ***\n")
        stop_event.set()
        collection_thread.join(timeout=10)
        location_thread.join(timeout=5)

        # Stop traffic capture
        info("*** Stopping traffic capture ***\n")
        for drone in drones:
            traffic_analyzer.stop_capture(drone)

        # Analyze traffic
        info("*** Analyzing traffic ***\n")
        traffic_stats = traffic_analyzer.analyze_all([d.name for d in drones])

        # Close metrics collector
        collector.close()

        # Generate experiment report
        self._generate_report(output_dir, experiment, traffic_stats, collector)

        # Cleanup
        info("*** Cleaning up ***\n")
        self._cleanup_drones()
        net.stop()

    def _setup_topology(self, params: Dict):
        """Setup Mininet-WiFi topology with experiment parameters."""
        # Temporarily override config values
        import config

        original_values = {
            "DRONE_NUMBER": config.DRONE_NUMBER,
            "SPEED": config.SPEED,
            "MOBILITY_MODEL": config.MOBILITY_MODEL,
        }

        config.DRONE_NUMBER = params["drone_count"]
        config.MOBILITY_MODEL = params.get("mobility_model", "GaussMarkov")

        net, drones = setup_topology()
        net.build()
        net.start()

        # Wait for network interfaces to be fully ready
        time.sleep(3)

        # Restore original values
        for key, value in original_values.items():
            setattr(config, key, value)

        return net, drones

    def _apply_network_loss(self, drones, loss_percent: float):
        """Apply packet loss to drone interfaces."""
        info(f"*** Applying {loss_percent}% packet loss ***\n")
        for drone in drones:
            interface = f"{drone.name}-wlan0"
            cmd = f"tc qdisc add dev {interface} root netem loss {loss_percent}%"
            drone.cmd(cmd)

    def _start_drone_apps(self, drones, params: Dict):
        """Start Go drone applications on all nodes."""
        # Adjust intervals for simulation multiplier
        sample_interval = params["sample_interval_sec"] / SIMULATION_MULTIPLIER
        delta_push_interval = params["delta_push_interval_sec"] / SIMULATION_MULTIPLIER
        anti_entropy_interval = (
            params["anti_entropy_interval_sec"] / SIMULATION_MULTIPLIER
        )

        for i, drone in enumerate(drones, 1):
            drone_id = f"drone-go-{i}"
            command = (
                f"{EXEC_PATH} "
                f"-id={drone_id} "
                f"-sample-ms={int(sample_interval * 1000)} "
                f"-fanout={params['fanout']} "
                f"-ttl={params['ttl']} "
                f"-delta-push-ms={int(delta_push_interval * 1000)} "
                f"-anti-entropy-ms={int(anti_entropy_interval * 1000)} "
                f"-udp-port={UDP_PORT} "
                f"-tcp-port={TCP_PORT} "
                f"-bind={BIND_ADDR} "
                f"-hello-ms=1000 "
                f"-hello-jitter-ms=200 "
                f"-confidence-threshold=50.0 "
                f"> /tmp/{drone_id}.log 2>&1 &"
            )
            drone.cmd(command)
            info(f"  Started {drone_id}\n")

    def _collect_metrics_loop(self, drones, stop_event, collector, sample_interval_sec):
        """Main metrics collection loop with parallel fetching."""
        iteration = 0
        # Limit concurrent requests to avoid overwhelming drones
        max_workers = min(10, len(drones))

        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            while not stop_event.is_set():
                stop_event.wait(sample_interval_sec)
                if stop_event.is_set():
                    break

                timestamp = time.time()

                # Submit all drone fetches in parallel
                futures = {
                    executor.submit(self._fetch_drone_metrics, drone, timestamp): drone
                    for drone in drones
                }

                # Collect results as they complete
                for future in as_completed(futures, timeout=sample_interval_sec * 0.8):
                    drone = futures[future]
                    try:
                        result = future.result()
                        if result:
                            stats, state = result
                            if stats:
                                collector.record_metrics(
                                    drone.name, timestamp, stats, state
                                )
                    except Exception as e:
                        # Error already logged in _fetch_drone_metrics
                        pass

                iteration += 1
                if iteration % 10 == 0:
                    info(f"  Collected {iteration} samples\n")

    def _fetch_drone_metrics(self, drone, timestamp):
        """Fetch both stats and state from a drone (for parallel execution)."""
        try:
            # Add small random jitter to avoid thundering herd
            time.sleep(random.uniform(0, 0.1))
            stats = self._fetch_drone_stats(drone)
            state = self._fetch_drone_state(drone)
            return (stats, state)
        except Exception as e:
            info(f"Error fetching metrics from {drone.name}: {e}\n")
            return None

    def _fetch_drone_stats(self, drone) -> Dict:
        """Fetch comprehensive stats from a drone."""
        cmd = f"curl -s --max-time 2 http://{drone.IP()}:{TCP_PORT}/stats 2>/dev/null"
        try:
            response_str = drone.cmd(cmd).strip()
            return json.loads(response_str)
        except (json.JSONDecodeError, Exception):
            return None

    def _fetch_drone_state(self, drone) -> Dict:
        """Fetch state info from a drone."""
        cmd = f"curl -s --max-time 2 http://{drone.IP()}:{TCP_PORT}/state 2>/dev/null"
        try:
            response_str = drone.cmd(cmd).strip()
            return json.loads(response_str)
        except (json.JSONDecodeError, Exception):
            return None

    def _cleanup_drones(self):
        """Kill all drone processes."""
        os.system(f"sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null")

    def _cleanup_mininet(self):
        """Clean up any existing Mininet processes and controllers."""
        info("*** Cleaning up existing Mininet processes ***\n")

        # Kill any existing controller processes
        os.system("sudo pkill -9 -f 'controller' 2>/dev/null")
        os.system("sudo fuser -k 6653/tcp 2>/dev/null")

        # Kill any lingering tcpdump processes
        os.system("sudo pkill -9 tcpdump 2>/dev/null")

        # Clean up any tc (traffic control) rules on all wireless interfaces
        # Use a more generic approach to catch all sta*-wlan0 interfaces
        os.system(
            "for iface in $(ip link show | grep -o 'sta[0-9]*-wlan0'); do sudo tc qdisc del dev $iface root 2>/dev/null; done"
        )

        # Run mn -c to clean up Mininet
        os.system("sudo mn -c > /dev/null 2>&1")

        # Additional cleanup for Open vSwitch
        os.system("sudo ovs-vsctl del-br s1 2>/dev/null")
        os.system("sudo ovs-vsctl del-br s2 2>/dev/null")
        os.system("sudo ovs-vsctl del-br s3 2>/dev/null")

        # Wait a moment for cleanup to complete
        time.sleep(3)

        info("*** Cleanup complete ***\n")

    def _generate_report(self, output_dir, experiment, traffic_stats, collector):
        """Generate experiment summary report."""
        report_file = output_dir / "REPORT.txt"

        with open(report_file, "w") as f:
            f.write("=" * 80 + "\n")
            f.write(f"EXPERIMENT REPORT: {experiment['id']}\n")
            f.write("=" * 80 + "\n\n")

            f.write("DESCRIPTION\n")
            f.write("-" * 80 + "\n")
            f.write(f"{experiment['description']}\n\n")

            f.write("PARAMETERS\n")
            f.write("-" * 80 + "\n")
            for key, value in experiment["parameters"].items():
                f.write(f"  {key:.<40} {value}\n")
            f.write("\n")

            f.write("OUTPUT FILES\n")
            f.write("-" * 80 + "\n")
            f.write(f"  Experiment config ........ experiment.json\n")
            f.write(f"  Metrics data ............. metrics.csv\n")
            f.write(f"  Traffic analysis ......... traffic/traffic_analysis.json\n")
            f.write(f"  Traffic summary .......... traffic/traffic_summary.txt\n")
            f.write(f"  Packet captures .......... traffic/pcaps/*.pcap\n")
            f.write("\n")

            if traffic_stats:
                total_bytes = sum(s["total_bytes"] for s in traffic_stats.values())
                f.write("TRAFFIC SUMMARY\n")
                f.write("-" * 80 + "\n")
                f.write(f"  Total bytes .............. {total_bytes:,}\n")
                f.write(
                    f"  Total MB ................. {total_bytes / 1024 / 1024:.2f}\n"
                )
                f.write(
                    f"  Avg per drone ............ {total_bytes / len(traffic_stats):,.0f} bytes\n\n"
                )

            f.write("=" * 80 + "\n")

        info(f"Report saved to {report_file}\n")


class MetricsCollector:
    """Collects and stores comprehensive metrics in the desired format."""

    def __init__(self, output_dir: str, scenario_id: str, sample_interval: int):
        self.output_dir = Path(output_dir)
        self.scenario_id = scenario_id
        self.sample_interval = sample_interval

        # Open CSV file
        self.csv_file = open(self.output_dir / "metrics.csv", "w", newline="")
        self.csv_writer = csv.writer(self.csv_file)

        # Write header
        self.csv_writer.writerow(
            [
                "t",
                "drone_id",
                "scenario_id",
                # Position
                "pos_x",
                "pos_y",
                # Network metrics
                "msgs_sent_total",
                "msgs_recv_total",
                "duplicates_dropped",
                "dedup_cache_size",
                # CRDT metrics
                "active_elements",
                "state_entries",
                # Dissemination metrics
                "delta_messages_sent",
                "anti_entropy_messages_sent",
                # Neighbor metrics
                "neighbor_count",
                # Raw stats (JSON)
                "raw_stats",
            ]
        )

    def record_metrics(
        self, drone_id: str, timestamp: float, stats: Dict, state: Dict = None
    ):
        """Record metrics from a drone's /stats and /state endpoints."""
        # Extract nested stats
        dissemination = stats.get("dissemination", {})
        network = stats.get("network", {})
        sensor = stats.get("sensor_system", {})

        # Get position from sensor stats
        position = sensor.get("position", {})
        pos_x = position.get("x", 0)
        pos_y = position.get("y", 0)

        # Get state info from /state endpoint
        if state:
            # total_deltas is the number of active fire cells
            active_elements = state.get("total_deltas", 0)
            # unique_sensors is the number of different drones that detected fires
            state_entries = state.get("unique_sensors", 0)
        else:
            active_elements = 0
            state_entries = 0

        # Get dissemination metrics
        delta_messages_sent = dissemination.get("delta_messages_sent", 0)
        anti_entropy_sent = dissemination.get("anti_entropy_count", 0)

        # Get neighbor count - it's the length of neighbor_ids array in network stats
        neighbor_ids = network.get("neighbor_ids", [])
        neighbor_count = (
            len(neighbor_ids)
            if isinstance(neighbor_ids, list)
            else network.get("neighbors_active", 0)
        )

        # Write row
        self.csv_writer.writerow(
            [
                timestamp,
                drone_id,
                self.scenario_id,
                # Position
                pos_x,
                pos_y,
                # Network
                dissemination.get("sent_count", 0),
                dissemination.get("received_count", 0),
                dissemination.get("dropped_count", 0),
                dissemination.get("cache_size", 0),
                # CRDT
                active_elements,
                state_entries,
                # Dissemination
                delta_messages_sent,
                anti_entropy_sent,
                # Network
                neighbor_count,
                # Raw
                json.dumps(stats),
            ]
        )

        # Flush to disk
        self.csv_file.flush()

    def close(self):
        """Close the CSV file."""
        if self.csv_file:
            self.csv_file.close()


def main():
    """Main entry point."""
    try:
        os.system(f"sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null")
    except:
        pass

    setLogLevel("info")

    runner = ExperimentRunner("experiments.json")
    runner.run_all_experiments()


if __name__ == "__main__":
    main()

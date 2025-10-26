"""
Network traffic analysis using tcpdump - no Go code changes needed.
Captures and analyzes packet-level metrics from Mininet-WiFi interfaces.
"""

import json
import subprocess
from pathlib import Path
from typing import Dict, List


class TrafficAnalyzer:
    """Analyze network traffic using tcpdump/tshark."""

    def __init__(self, output_dir: str):
        self.output_dir = Path(output_dir)
        self.output_dir.mkdir(parents=True, exist_ok=True)
        self.pcap_dir = self.output_dir / "pcaps"
        self.pcap_dir.mkdir(exist_ok=True)

    def start_capture(self, drone, tcp_port: int = 8080, udp_port: int = 7000):
        """Start tcpdump capture on drone's interface."""
        interface = f"{drone.name}-wlan0"
        pcap_file = self.pcap_dir / f"{drone.name}.pcap"

        # Capture both UDP (HELLO) and TCP (state sync) traffic
        filter_expr = f"udp port {udp_port} or tcp port {tcp_port}"

        cmd = (
            f"tcpdump -i {interface} -w {pcap_file} "
            f"-s 65535 '{filter_expr}' 2>/dev/null &"
        )

        drone.cmd(cmd)
        print(f"Started capture on {drone.name} -> {pcap_file}")

    def stop_capture(self, drone):
        """Stop tcpdump on drone."""
        drone.cmd(f"pkill -f 'tcpdump -i {drone.name}-wlan0'")

    def analyze_pcap(self, drone_name: str) -> Dict:
        """
        Analyze captured pcap file using tshark.
        Returns breakdown of traffic by type, sizes, counts.
        Extracts custom headers to identify message types.
        """
        pcap_file = self.pcap_dir / f"{drone_name}.pcap"
        if not pcap_file.exists():
            return {}

        results = {
            "total_packets": 0,
            "total_bytes": 0,
            "udp_packets": 0,
            "udp_bytes": 0,
            "tcp_packets": 0,
            "tcp_bytes": 0,
            "avg_packet_size": 0,
            "packet_sizes": [],
            # Message type breakdown from custom headers
            "by_message_type": {
                "DELTA": {"count": 0, "bytes": 0},
                "ANTI-ENTROPY": {"count": 0, "bytes": 0},
                "STATE": {"count": 0, "bytes": 0},
                "STATS": {"count": 0, "bytes": 0},
                "POSITION": {"count": 0, "bytes": 0},
                "HELLO": {"count": 0, "bytes": 0},  # UDP multicast
                "UNKNOWN": {"count": 0, "bytes": 0},
            },
        }

        # Get packet details including HTTP headers
        # For custom headers, we need to use http.request.line or parse the full HTTP data
        cmd = [
            "tshark",
            "-r",
            str(pcap_file),
            "-Y",
            "http or udp",  # Filter for HTTP and UDP
            "-T",
            "fields",
            "-e",
            "frame.len",
            "-e",
            "ip.proto",
            "-e",
            "http.request.uri",
            "-e",
            "http.response",
            "-e",
            "http.request.method",
            "-e",
            "udp.port",
            "-E",
            "separator=|",
        ]

        try:
            output = subprocess.check_output(cmd, stderr=subprocess.DEVNULL).decode()

            # Extract X-Message-Type headers from HTTP packets using a separate query
            # This searches for the custom header in the HTTP data
            header_cmd = [
                "tshark",
                "-r",
                str(pcap_file),
                "-Y",
                "http.request",
                "-T",
                "fields",
                "-e",
                "frame.number",
                "-e",
                "http.file_data",
                "-E",
                "separator=|",
            ]

            # Build a map of frame number to message type
            frame_to_msg_type = {}
            try:
                header_output = subprocess.check_output(
                    header_cmd, stderr=subprocess.DEVNULL
                ).decode()
                for hline in header_output.strip().split("\n"):
                    if not hline or "|" not in hline:
                        continue
                    hparts = hline.split("|", 1)
                    frame_num = hparts[0]
                    http_data = hparts[1] if len(hparts) > 1 else ""

                    # Look for X-Message-Type in the HTTP data
                    if "X-Message-Type:" in http_data:
                        # Extract the value after X-Message-Type:
                        for line in http_data.split("\\n"):
                            if "X-Message-Type:" in line:
                                msg_type = (
                                    line.split("X-Message-Type:")[-1].strip().split()[0]
                                )
                                frame_to_msg_type[frame_num] = msg_type.upper()
                                break
            except subprocess.CalledProcessError:
                pass  # If we can't extract headers, fall back to URI inspection

            frame_num = 0
            for line in output.strip().split("\n"):
                if not line:
                    continue
                frame_num += 1
                parts = line.split("|")
                if len(parts) < 2:
                    continue

                size = int(parts[0]) if parts[0] else 0
                proto = parts[1] if len(parts) > 1 else ""
                uri = parts[2] if len(parts) > 2 else ""
                # parts[3] is http.response
                # parts[4] is http.request.method
                udp_port = parts[5] if len(parts) > 5 else ""

                results["total_packets"] += 1
                results["total_bytes"] += size
                results["packet_sizes"].append(size)

                # Determine message type
                msg_type = "UNKNOWN"

                if proto == "17":  # UDP
                    results["udp_packets"] += 1
                    results["udp_bytes"] += size
                    # UDP on port 7000 is likely HELLO multicast
                    if "7000" in udp_port:
                        msg_type = "HELLO"

                elif proto == "6":  # TCP
                    results["tcp_packets"] += 1
                    results["tcp_bytes"] += size

                    # First try to get from extracted headers
                    frame_key = str(frame_num)
                    if frame_key in frame_to_msg_type:
                        msg_type = frame_to_msg_type[frame_key]
                    # Fallback to URI inspection
                    elif "/delta" in uri:
                        # Check if it's DELTA or ANTI-ENTROPY (both use /delta endpoint)
                        msg_type = "DELTA"  # Will be overridden by header if available
                    elif "/state" in uri:
                        msg_type = "STATE"
                    elif "/stats" in uri:
                        msg_type = "STATS"
                    elif "/position" in uri:
                        msg_type = "POSITION"

                # Update message type counters
                if msg_type in results["by_message_type"]:
                    results["by_message_type"][msg_type]["count"] += 1
                    results["by_message_type"][msg_type]["bytes"] += size
                else:
                    results["by_message_type"]["UNKNOWN"]["count"] += 1
                    results["by_message_type"]["UNKNOWN"]["bytes"] += size

            if results["total_packets"] > 0:
                results["avg_packet_size"] = (
                    results["total_bytes"] / results["total_packets"]
                )

        except subprocess.CalledProcessError as e:
            print(f"Warning: Could not analyze {pcap_file}: {e}")

        return results

    def analyze_all(self, drone_names: List[str]) -> Dict:
        """Analyze all drone pcaps and return aggregated stats."""
        all_stats = {}

        for name in drone_names:
            stats = self.analyze_pcap(name)
            if stats:
                all_stats[name] = stats

        # Save to JSON
        output_file = self.output_dir / "traffic_analysis.json"
        with open(output_file, "w") as f:
            json.dump(all_stats, f, indent=2)

        print(f"Traffic analysis saved to {output_file}")

        # Generate summary report
        self.generate_summary_report(all_stats)

        return all_stats

    def generate_summary_report(self, all_stats: Dict):
        """Generate a human-readable summary of traffic analysis."""
        report_file = self.output_dir / "traffic_summary.txt"

        with open(report_file, "w") as f:
            f.write("=" * 80 + "\n")
            f.write("TRAFFIC ANALYSIS SUMMARY\n")
            f.write("=" * 80 + "\n\n")

            # Aggregate across all drones
            total_packets = sum(s["total_packets"] for s in all_stats.values())
            total_bytes = sum(s["total_bytes"] for s in all_stats.values())

            f.write(f"Total drones analyzed: {len(all_stats)}\n")
            f.write(f"Total packets captured: {total_packets:,}\n")
            f.write(
                f"Total bytes transferred: {total_bytes:,} ({total_bytes/1024/1024:.2f} MB)\n\n"
            )

            # Message type breakdown
            f.write("-" * 80 + "\n")
            f.write("MESSAGE TYPE BREAKDOWN\n")
            f.write("-" * 80 + "\n")

            msg_type_totals = {}
            for drone_stats in all_stats.values():
                for msg_type, stats in drone_stats.get("by_message_type", {}).items():
                    if msg_type not in msg_type_totals:
                        msg_type_totals[msg_type] = {"count": 0, "bytes": 0}
                    msg_type_totals[msg_type]["count"] += stats["count"]
                    msg_type_totals[msg_type]["bytes"] += stats["bytes"]

            f.write(
                f"{'Type':<20} {'Count':>12} {'Bytes':>15} {'% Packets':>12} {'% Bytes':>12}\n"
            )
            f.write("-" * 80 + "\n")

            for msg_type in sorted(msg_type_totals.keys()):
                stats = msg_type_totals[msg_type]
                pct_packets = (
                    (stats["count"] / total_packets * 100) if total_packets > 0 else 0
                )
                pct_bytes = (
                    (stats["bytes"] / total_bytes * 100) if total_bytes > 0 else 0
                )

                f.write(
                    f"{msg_type:<20} {stats['count']:>12,} {stats['bytes']:>15,} "
                    f"{pct_packets:>11.2f}% {pct_bytes:>11.2f}%\n"
                )

            # Per-drone summary
            f.write("\n" + "-" * 80 + "\n")
            f.write("PER-DRONE SUMMARY\n")
            f.write("-" * 80 + "\n")
            f.write(
                f"{'Drone':<10} {'Packets':>10} {'Bytes':>12} {'UDP%':>8} {'TCP%':>8} {'Avg Size':>10}\n"
            )
            f.write("-" * 80 + "\n")

            for drone_name, stats in sorted(all_stats.items()):
                udp_pct = (
                    (stats["udp_packets"] / stats["total_packets"] * 100)
                    if stats["total_packets"] > 0
                    else 0
                )
                tcp_pct = (
                    (stats["tcp_packets"] / stats["total_packets"] * 100)
                    if stats["total_packets"] > 0
                    else 0
                )

                f.write(
                    f"{drone_name:<10} {stats['total_packets']:>10,} {stats['total_bytes']:>12,} "
                    f"{udp_pct:>7.1f}% {tcp_pct:>7.1f}% {stats['avg_packet_size']:>9.1f}\n"
                )

            f.write("\n" + "=" * 80 + "\n")

        print(f"Summary report saved to {report_file}")

        # Also print to console
        with open(report_file, "r") as f:
            print("\n" + f.read())

    def get_bandwidth_utilization(self, drone_name: str, duration_sec: float) -> float:
        """Calculate average bandwidth in bytes/sec."""
        stats = self.analyze_pcap(drone_name)
        if not stats or duration_sec <= 0:
            return 0.0
        return stats["total_bytes"] / duration_sec


def example_usage_in_simulation(net, drones, duration_sec: int = 300):
    """
    Example of how to use traffic capture in your simulation.

    Add this to your main.py:

    1. After net.start():
       analyzer = TrafficAnalyzer("metrics_output/current_run")
       for drone in drones:
           analyzer.start_capture(drone)

    2. Before net.stop():
       for drone in drones:
           analyzer.stop_capture(drone)

       stats = analyzer.analyze_all([d.name for d in drones])
    """
    pass


# What you get from this approach:
#
# ✓ Total bytes/packets sent per drone (ground truth)
# ✓ UDP vs TCP breakdown (HELLO vs state sync)
# ✓ Packet size distributions
# ✓ Actual bandwidth utilization
# ✓ Retransmission detection (TCP analysis)
# ✓ Inter-packet timing (with -e frame.time_delta)
#
# Limitations:
# ✗ Can't distinguish DELTA from AE messages (both use TCP)
#   - Unless you add HTTP path inspection with tshark filters
# ✗ No semantic info (delta IDs, hop counts, etc.)
# ✗ Higher storage overhead than app-level logging

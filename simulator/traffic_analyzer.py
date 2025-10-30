"""
Network traffic analysis using tcpdump - no Go code changes needed.
Captures and analyzes packet-level metrics from Mininet-WiFi interfaces.
"""

import json
import re
import subprocess
from pathlib import Path
from typing import Dict, List


class TrafficAnalyzer:
    """
    Analyze network traffic using tcpdump/tshark.

    This class captures and analyzes packet-level metrics from Mininet-WiFi interfaces,
    providing detailed statistics on UDP/TCP traffic, message types, and request/response patterns.

    Main workflow:
    1. start_capture() - Begin packet capture on drone interfaces
    2. stop_capture() - Stop capture gracefully
    3. analyze_pcap() - Parse captured packets and extract metrics
    4. analyze_all() - Aggregate statistics across all drones
    5. generate_summary_report() - Create human-readable summary
    """

    def __init__(self, output_dir: str):
        self.output_dir = Path(output_dir)
        self.output_dir.mkdir(parents=True, exist_ok=True)
        self.pcap_dir = self.output_dir / "pcaps"
        self.pcap_dir.mkdir(exist_ok=True)

    def start_capture(self, drone, tcp_port: int = 8080, udp_port: int = 7000):
        """Start tcpdump capture on drone's interface."""
        interface = f"{drone.name}-wlan0"
        pcap_file = self.pcap_dir / f"{drone.name}.pcap"

        cmd = f"tcpdump -i {interface} -w {pcap_file}"

        drone.cmd(f'xterm -e "{cmd}" 2>/dev/null &')
        print(f"Started capture on {drone.name} -> {pcap_file}")

    def stop_capture(self, drone):
        """Stop tcpdump on drone gracefully to avoid corrupted pcap files."""
        # Send SIGTERM first to allow tcpdump to flush buffers
        drone.cmd(f"pkill -TERM -f 'tcpdump -i {drone.name}-wlan0'")
        # Give it a moment to flush
        import time

        time.sleep(0.5)
        # Force kill any remaining processes
        drone.cmd(f"pkill -9 -f 'tcpdump -i {drone.name}-wlan0'")

    def _validate_pcap_file(self, pcap_file: Path) -> bool:
        """Validate that pcap file exists and is not corrupted."""
        if not pcap_file.exists():
            return False

        file_size = pcap_file.stat().st_size
        if file_size == 0:
            print(f"Warning: {pcap_file} is empty, skipping analysis")
            return False
        if file_size < 24:  # Minimum pcap header size
            print(
                f"Warning: {pcap_file} is too small ({file_size} bytes), likely corrupted"
            )
            return False

        return True

    def _initialize_results(self) -> Dict:
        """Initialize the results dictionary structure."""
        message_types = [
            "DELTA",
            "ANTI-ENTROPY",
            "STATE",
            "STATS",
            "POSITION",
            "HELLO",
            "UNKNOWN",
        ]

        return {
            "total_packets": 0,
            "total_bytes": 0,
            "udp_packets": 0,
            "udp_bytes": 0,
            "tcp_packets": 0,
            "tcp_bytes": 0,
            "avg_packet_size": 0,
            "packet_sizes": [],
            "by_message_type": {
                msg_type: {
                    "count": 0,
                    "bytes": 0,
                    "requests": 0,
                    "responses": 0,
                    "percentage_unresponded": 0.0,
                }
                for msg_type in message_types
            },
        }

    def analyze_pcap(self, drone_name: str) -> Dict:
        """
        Analyze captured pcap file using tshark.
        Returns breakdown of traffic by type, sizes, counts.
        Extracts custom headers to identify message types.
        """
        pcap_file = self.pcap_dir / f"{drone_name}.pcap"

        if not self._validate_pcap_file(pcap_file):
            return {}

        results = self._initialize_results()

        # Analyze UDP and HTTP/TCP packets
        self._analyze_udp_packets(pcap_file, results)
        self._analyze_http_packets(pcap_file, results)

        # Finalize calculations
        self._calculate_statistics(results)

        return results

    def _analyze_udp_packets(self, pcap_file: Path, results: Dict):
        """Analyze UDP packets from pcap file."""
        udp_cmd = [
            "tshark",
            "-r",
            str(pcap_file),
            "-Y",
            "udp",
            "-T",
            "fields",
            "-e",
            "frame.number",
            "-e",
            "frame.len",
            "-E",
            "separator=|",
        ]

        try:
            result = subprocess.run(udp_cmd, capture_output=True, text=True, check=True)

            for line in result.stdout.strip().split("\n"):
                if not line:
                    continue

                parts = line.split("|")
                if len(parts) < 2:
                    continue

                size = int(parts[1]) if parts[1] else 0

                # Update global counters
                self._update_packet_stats(results, size, is_udp=True)

                # UDP packets are HELLO multicast
                self._update_message_type_stats(results, "HELLO", size, is_request=True)

        except subprocess.CalledProcessError:
            print(f"Warning: Could not analyze UDP packets in {pcap_file}")

    def _analyze_http_packets(self, pcap_file: Path, results: Dict):
        """Analyze HTTP/TCP packets from pcap file."""
        http_cmd = [
            "tshark",
            "-r",
            str(pcap_file),
            "-Y",
            "http",
            "-T",
            "fields",
            "-e",
            "frame.number",
            "-e",
            "frame.len",
            "-e",
            "http.response_for.uri",
            "-e",
            "http.request.line",
            "-e",
            "http.response.line",
            "-E",
            "separator=|",
        ]

        try:
            result = subprocess.run(
                http_cmd, capture_output=True, text=True, check=True
            )
            frame_metadata = self._extract_frame_metadata(result.stdout)

            for line in result.stdout.strip().split("\n"):
                if not line:
                    continue

                parts = line.split("|")
                if len(parts) < 2:
                    continue

                frame_key = parts[0].strip()
                size = int(parts[1]) if parts[1] else 0

                # Update global counters
                self._update_packet_stats(results, size, is_udp=False)

                # Get message type and response status from custom headers only
                msg_type = frame_metadata.get(frame_key, {}).get("msg_type", "UNKNOWN")
                is_response = frame_metadata.get(frame_key, {}).get(
                    "is_response", False
                )

                # Update message type statistics
                self._update_message_type_stats(
                    results, msg_type, size, is_request=not is_response
                )

        except subprocess.CalledProcessError as e:
            self._handle_pcap_error(pcap_file, e)

    def _extract_frame_metadata(self, tshark_output: str) -> Dict:
        """Extract message type and request/response status from HTTP headers."""
        frame_metadata = {}

        for line in tshark_output.strip().split("\n"):
            if not line:
                continue

            parts = line.split("|")
            if len(parts) < 4:
                continue

            frame_key = parts[0].strip()
            http_request_value = parts[3] if len(parts) > 3 else ""
            http_response_value = parts[4] if len(parts) > 4 else ""

            # Process request headers
            if http_request_value:
                for header_line in http_request_value.split("\r\n,"):
                    msg_type = retrieve_message_type(header_line)
                    if msg_type:
                        frame_metadata[frame_key] = {
                            "msg_type": msg_type,
                            "is_response": False,
                        }
                        break

            # Process response headers (overrides request if both exist)
            if http_response_value:
                for header_line in http_response_value.split("\r\n,"):
                    msg_type = retrieve_message_type(header_line)
                    if msg_type:
                        frame_metadata[frame_key] = {
                            "msg_type": msg_type,
                            "is_response": True,
                        }
                        break

        return frame_metadata

    def _update_packet_stats(self, results: Dict, size: int, is_udp: bool):
        """Update global packet statistics."""
        results["total_packets"] += 1
        results["total_bytes"] += size
        results["packet_sizes"].append(size)

        if is_udp:
            results["udp_packets"] += 1
            results["udp_bytes"] += size
        else:
            results["tcp_packets"] += 1
            results["tcp_bytes"] += size

    def _update_message_type_stats(
        self, results: Dict, msg_type: str, size: int, is_request: bool
    ):
        """Update message type specific statistics."""
        if msg_type not in results["by_message_type"]:
            msg_type = "UNKNOWN"

        results["by_message_type"][msg_type]["count"] += 1
        results["by_message_type"][msg_type]["bytes"] += size

        if is_request:
            results["by_message_type"][msg_type]["requests"] += 1
        else:
            results["by_message_type"][msg_type]["responses"] += 1

    def _calculate_statistics(self, results: Dict):
        """Calculate derived statistics like averages and percentages."""
        # Calculate average packet size
        if results["total_packets"] > 0:
            results["avg_packet_size"] = (
                results["total_bytes"] / results["total_packets"]
            )

        # Calculate percentage of unresponded requests for each message type
        for msg_type, stats in results["by_message_type"].items():
            if stats["requests"] > 0:
                unresponded = stats["requests"] - stats["responses"]
                stats["percentage_unresponded"] = (
                    unresponded / stats["requests"]
                ) * 100.0
            else:
                stats["percentage_unresponded"] = 0.0

    def _handle_pcap_error(self, pcap_file: Path, error: subprocess.CalledProcessError):
        """Handle errors when analyzing pcap files."""
        stderr_msg = error.stderr if hasattr(error, "stderr") and error.stderr else ""

        if "cut short" in stderr_msg or "appears to have been cut short" in stderr_msg:
            print(
                f"Warning: {pcap_file.name} was corrupted (cut short), attempting recovery..."
            )
            try:
                simple_cmd = [
                    "tshark",
                    "-r",
                    str(pcap_file),
                    "-q",
                    "-z",
                    "io,stat,0",
                ]
                result = subprocess.run(simple_cmd, capture_output=True, text=True)
                if "captured" in result.stdout:
                    print(
                        "  File partially readable but too corrupted for detailed analysis"
                    )
            except:
                pass
        else:
            print(f"Warning: Could not analyze {pcap_file}")
            print(f"  Error: {error}")
            if stderr_msg:
                print(f"  Stderr: {stderr_msg}")

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


def retrieve_message_type(line):
    match = re.search(r"X-Message-Type:\s*([^\\]+)", line)

    if match:
        # match.group(0) is the entire match (e.g., "X-Message-Type: DELTA")
        # match.group(1) is the first capturing group (e.g., "DELTA")
        return match.group(1).strip()
    else:
        return None

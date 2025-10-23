import csv
import os
import threading
import time

from config import (
    BIND_ADDR,
    EXEC_PATH,
    FANOUT,
    OUTPUT_DIR,
    TCP_PORT,
    UDP_PORT,
    anti_entropy_interval,
    confidence_threshold,
    delta_push_interval,
    duration,
    hello_interval_ms,
    hello_jitter_ms,
    neighbor_timeout_sec,
    sample_interval_sec,
    ttl,
)
from drone_ui import setup_UI
from drone_utils import fetch_states, send_locations, setup_topology
from mininet.log import info, setLogLevel
from mn_wifi.cli import CLI


def main():
    """Main execution function."""
    try:
        os.system(f"sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null")
    except:
        pass

    setLogLevel("info")

    net, drones = setup_topology()
    # net.telemetry(nodes=drones, single=True, data_type='tx_bytes', title="FANET TX BYTES")
    net.plotGraph()

    info("*** Building the network ***\n")
    net.build()
    net.start()

    info("--- Starting Go applications on drones... ---\n")
    for i, drone in enumerate(net.stations, 1):
        drone_id = f"drone-go-{i}"
        command = (
            f"{EXEC_PATH} "
            f"-id={drone_id} "
            f"-sample-sec={int(sample_interval_sec)} "
            f"-fanout={FANOUT} "
            f"-ttl={int(ttl)} "
            f"-delta-push-sec={int(delta_push_interval)} "
            f"-anti-entropy-sec={int(anti_entropy_interval)} "
            f"-udp-port={UDP_PORT} "
            f"-tcp-port={TCP_PORT} "
            f"-bind={BIND_ADDR} "
            f"-hello-ms={int(hello_interval_ms)} "
            f"-hello-jitter-ms={int(hello_jitter_ms)} "
            f"-confidence-threshold={confidence_threshold} "
        )
        drone.cmd(f'xterm -e "{command}" &')

    info("\n*** Simulation is running. Type 'exit' or Ctrl+D to quit. ***\n")
    csv_files = {}
    csv_writers = {}

    # Use a try...finally block to ensure files are always closed properly.
    try:
        for drone in drones:
            filename = os.path.join(OUTPUT_DIR, f"{drone.name}_data.csv")
            # Open file in write mode with newline='' to prevent blank rows
            file_handle = open(filename, "w", newline="")
            writer = csv.writer(file_handle)
            # Write the header row
            writer.writerow(
                [
                    "timestamp",
                    "all_deltas",
                    "confidence",
                    "position",
                    "repetition",
                    "convergence",
                ]
            )

            csv_files[drone.name] = file_handle
            csv_writers[drone.name] = writer
            info(f"Opened {filename} for data logging.\n")

        stop_event = threading.Event()

        fetch_thread = threading.Thread(
            target=fetch_states, args=(drones, stop_event, csv_writers), daemon=True
        )
        fetch_thread.start()

        time.sleep(duration / 2)  # Ensure fetch thread starts before sending locations

        send_thread = threading.Thread(
            target=send_locations, args=(drones, stop_event), daemon=True
        )
        send_thread.start()

        running_ui_thread = threading.Thread(
            target=setup_UI, args=(drones,), daemon=True
        )
        running_ui_thread.start()

        info(
            "\n*** Simulation is running. CSV data is being saved in 'drone_execution_data'. ***\n"
        )
        info("*** Type 'exit' or Ctrl+D in the CLI to quit. ***\n")
        CLI(net)

    finally:
        info("*** Shutting down simulation ***\n")
        if "stop_event" in locals():
            stop_event.set()
        if "fetch_thread" in locals():
            fetch_thread.join(timeout=5)
        if "running_ui_thread" in locals():
            running_ui_thread.join(timeout=5)

        # Close all open CSV files
        for file_handle in csv_files.values():
            file_handle.close()
        info("Closed all data log files.\n")

    info("*** Shutting down simulation ***\n")
    net.stop()


if __name__ == "__main__":
    main()
    # Final cleanup of any lingering Go processes
    os.system(f"sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null")
    info("--- Simulation finished ---\n")

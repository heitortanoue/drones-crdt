import time
import os
import shutil
import threading
from datetime import datetime
import requests
import json

from mininet.node import Controller
from mininet.log import setLogLevel, info
from mn_wifi.link import wmediumd, adhoc
from mn_wifi.net import Mininet_wifi
from mn_wifi.cli import CLI
from mn_wifi.telemetry import telemetry 
from mn_wifi.wmediumdConnector import interference

# --- Configuration Constants ---
DRONE_NUMBER = 3
EXEC_PATH = '../drone/bin/drone-linux'  # Path to the compiled Go drone application
OUTPUT_DIR = 'drone_execution_data/'   # Directory for telemetry logs
TCP_PORT = 8080
UDP_PORT = 7000
POLL_INTERVAL = 1  # Telemetry polling interval in seconds
DRONE_NAMES = [f'dr{i}' for i in range(1, DRONE_NUMBER + 1)]

def update_telemetry_data_dir(names):
    """Moves generated telemetry files to the output directory"""
    info(f"*** Moving telemetry files to {OUTPUT_DIR}... ***\n")

    os.makedirs(OUTPUT_DIR, exist_ok=True)
    for name in names:
        source_file = f"position-{name}-mn-telemetry.txt"
        destination_file = os.path.join(OUTPUT_DIR, source_file)

        if os.path.exists(source_file):
            shutil.move(source_file, destination_file)
            info(f"-> File {source_file} moved <-\n")

def custom_telemetry_logger(nodes, output_dir, stop_event):
    """
    Polls drone positions and fetches the delta from the Go application's
    state endpoint via curl/HTTP GET. Logs to CSV files.
    """
    info("--- Starting Custom Telemetry Logger (for CSV files) ---\n")
    log_files = {}

    # Prepare log files and write headers
    for node in nodes:
        log_path = os.path.join(output_dir, f"telemetry-{node.name}.csv")
        log_files[node.name] = open(log_path, 'w')
        # Updated header to reflect the source of the delta
        log_files[node.name].write("timestamp,node,x,y,z,state_deltas\n")
        log_files[node.name].flush()

    while not stop_event.is_set():
        current_time = datetime.now().isoformat()

        for node in nodes:
            pos = node.position
            delta = 0.0  # Default delta if request fails

            # TODO: retrieve the deltas without running new cmd each time
            # Log format: timestamp, node_name, x, y, z, delta
            log_line = f"{current_time},{node.name},{pos[0]},{pos[1]},{pos[2]},{delta:.4f}\n"
            log_files[node.name].write(log_line)
            log_files[node.name].flush()

        time.sleep(POLL_INTERVAL)

    # Clean up and close all log files
    for f in log_files.values():
        f.close()
    info("--- Custom Telemetry Logger has stopped ---\n")


def setup_topology():
    """Creates and configures the network topology for the drone simulation."""
    info("--- Creating a Go drone network with Mininet-WiFi ---\n")

    net = Mininet_wifi(
        controller=Controller,
        link=wmediumd,
        wmediumd_mode=interference
    )
    net.addController('c0')

    # --- CHANGE 1: Create all station nodes first ---
    info("*** Creating drone nodes ***\n")
    drones = []
    for i, name in enumerate(DRONE_NAMES, 1):
        mac = f'00:00:00:00:00:0{i}'
        ip = f'10.0.0.{i}/8'
        drone = net.addStation(
            name,
            mac=mac,
            ip=ip,
            position='10,10,0',
            txpower=20
        )
        drones.append(drone) # Keep track of the created nodes

    info("*** Configuring the signal propagation model ***\n")
    net.setPropagationModel(model="logDistance", exp=4)

    # --- CHANGE 2: Configure nodes to create their interfaces ---
    info("*** Configuring network nodes ***\n")
    net.configureNodes()

    # --- CHANGE 3: Now, add the links to the configured nodes ---
    info("*** Adding ad-hoc links to drones ***\n")
    for drone in drones:
        net.addLink(
            drone,
            cls=adhoc,
            intf=f'{drone.name}-wlan0',
            ssid='adhocNet',
            proto='batman_adv',
            mode='g',
            channel=5,
            ht_cap='HT40+'
        )

    return net

def main():
    """Main execution function."""
    try:
        # Clean up any leftover Go processes from previous runs
        os.system(f'sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null')
    except:
        pass
    
    setLogLevel('info')
    
    net = setup_topology()
    
    info("*** Building the network ***\n")
    net.build()
    net.start()

    info("*** Configuring batman-adv interfaces on each drone ***\n")
    # This brings up the virtual mesh interface (bat0) and assigns the IP to it
    for station in net.stations:
        #station.cmd(f'ip link set up dev {station.name}-wlan0') # Ensure wlan is up
        station.cmd('ip link set up dev bat0')
        station.cmd(f'ip addr add {station.IP()}/8 dev bat0')
        station.cmd('ip route add default dev bat0')

    # --- STARTING TELEMETRY ---

    # 1. Start the custom logger for detailed CSV files (runs in a thread)
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    stop_event = threading.Event()
    telemetry_thread = threading.Thread(
        target=custom_telemetry_logger,
        args=(net.stations, OUTPUT_DIR, stop_event)
    )
    telemetry_thread.start()
    
    # 2. Start the built-in visualizer for a real-time plot (runs in a process)
    info("*** Starting real-time visualization plot... ***\n")
    telemetry(nodes=net.stations, single=True, data_type='position')

    # --- END OF TELEMETRY ---

    info("--- Starting Go applications on drones... ---\n")
    for i, drone in enumerate(net.stations, 1):
        drone_id = f'drone-go-{i}'
        command = (f"{EXEC_PATH} -id={drone_id} "
                   f"-tcp-port={TCP_PORT} -udp-port={UDP_PORT}")
        # xterm provides a separate terminal window for each drone's output
        drone.cmd(f'xterm -e "{command}" &')

    # Wait for Go applications to initialize
    time.sleep(5)

    info("\n*** Simulation is running. Type 'exit' or Ctrl+D to quit. ***\n")
    CLI(net)

    info("*** Shutting down simulation ***\n")
    # Gracefully stop the telemetry logger thread
    stop_event.set()
    telemetry_thread.join() # Wait for the thread to finish writing logs
    
    # The visualization plot started by telemetry() is stopped automatically by net.stop()
    net.stop()

if __name__ == '__main__':
    main()

    # Final cleanup of any lingering processes
    os.system(f'sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null')


    # Add a brief pause to ensure processes have time to receive the stop signal
    time.sleep(10)
    update_telemetry_data_dir(DRONE_NAMES)
    info("--- Simulation finished ---\n")
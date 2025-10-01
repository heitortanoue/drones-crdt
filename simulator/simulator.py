import os
import shutil
import threading
import json
import csv

from mininet.node import Controller
from mininet.log import setLogLevel, info
from mn_wifi.link import wmediumd, adhoc
from mn_wifi.net import Mininet_wifi
from mn_wifi.cli import CLI
from mn_wifi.wmediumdConnector import interference
from datetime import datetime

# --- Configuration Constants ---
DRONE_NUMBER = 4
EXEC_PATH = '../drone/bin/drone-linux'  # Path to the compiled Go drone application
OUTPUT_DIR = 'drone_execution_data/'   # Directory for telemetry logs
TCP_PORT = 8080
UDP_PORT = 7000
SPEED = 2
DRONE_NAMES = [f'dr{i}' for i in range(1, DRONE_NUMBER + 1)]
DRONE_IPs = []
duration = 10  # Duration to run the simulation before fetching data

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

def setup_topology():
    """Creates and configures the network topology for the drone simulation."""
    info("--- Creating a Go drone network with Mininet-WiFi ---\n")
    net = Mininet_wifi(
        controller=Controller,
        link=wmediumd,
        wmediumd_mode=interference
    )
    net.addController('c0')
    info("*** Creating drone nodes ***\n")
    drones = []
    kwargs = {}
    kwargs['range'] = 10
    for i, name in enumerate(DRONE_NAMES, 1):
        mac = f'00:00:00:00:00:0{i}'
        ip = f'10.0.0.{i}/8'
        DRONE_IPs.append(ip)
        drone = net.addStation(
            name,
            mac=mac,
            ip=ip,
            min_x=25 * (i-1), max_x=20 * i + 20,
            min_y=25 * (i-1), max_y=20 * i + 20,
            min_v=0.5*SPEED, max_v=SPEED, **kwargs
        )
        drones.append(drone)
    info("*** Configuring the signal propagation model ***\n")
    net.setPropagationModel(model="logDistance", exp=4)
    info("*** Configuring network nodes ***\n")
    net.configureNodes()
    net.plotGraph()
    net.setMobilityModel(time=0, model='RandomDirection',
                         max_x=100, max_y=100, seed=20)
    info("*** Adding ad-hoc links to drones ***\n")
    kwargs['proto'] = 'batman_adv'
    for drone in drones:
        net.addLink(
            drone,
            cls=adhoc,
            intf=f'{drone.name}-wlan0',
            ssid='adhocNet',
            mode='g',
            channel=5,
            ht_cap='HT40+', **kwargs
        )

    return net, drones

def fetch_states(drones, stop_event, csv_writers):
    """Fetches and logs the state of the drones periodically."""
    while not stop_event.is_set():
        stop_event.wait(duration)
        if stop_event.is_set():
            break
        for drone in drones:
            command = f'curl -s --max-time 5 http://{drone.IP()}:{TCP_PORT}/state'
            response_str = drone.cmd(command).strip()
            position = drone.position
            writer = csv_writers[drone.name]

            ## Parse the JSON response and log the specific fields.
            try:
                data = json.loads(response_str)
                
                # Extract data, assuming the first entry in latest_readings is the relevant one
                reading_data = list(data['latest_readings'].values())[0]
                timestamp_ms = reading_data['timestamp']
                confidence = reading_data['confidence']
                all_deltas = data['all_deltas']

                # Format the timestamp from milliseconds to a readable string
                formatted_timestamp = datetime.fromtimestamp(timestamp_ms / 1000).isoformat()
                
                # Convert all_deltas list to a compact JSON string for storage in a single CSV cell
                deltas_str = json.dumps(all_deltas)

                # Write the parsed data to the CSV file
                writer.writerow([formatted_timestamp, deltas_str, confidence, position])
                
            except (json.JSONDecodeError, KeyError, IndexError) as e:
                # Handle cases where the response is not valid JSON or missing keys
                error_timestamp = datetime.now().isoformat()
                error_msg = f"ERROR parsing response: {type(e).__name__}"
                writer.writerow([error_timestamp, error_msg, 'N/A', position])
                info(f"-> ERROR for {drone.name}: Could not parse JSON response. See CSV for details.\n")
                info(f"   Problematic response: {response_str}\n")

def main():
    """Main execution function."""
    try:
        os.system(f'sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null')
    except:
        pass
    
    setLogLevel('info')
    
    net, drones = setup_topology()
    
    info("*** Building the network ***\n")
    net.build()
    net.start()

    # info("*** Starting real-time visualization plot... ***\n")
    # telemetry(nodes=net.stations, single=True, data_type='position')

    info("--- Starting Go applications on drones... ---\n")
    for i, drone in enumerate(net.stations, 1):
        drone_id = f'drone-go-{i}'
        command = (f"{EXEC_PATH} -id={drone_id} "
                   f"-tcp-port={TCP_PORT} -udp-port={UDP_PORT}")
        drone.cmd(f'xterm -e "{command}" &')

    info("\n*** Simulation is running. Type 'exit' or Ctrl+D to quit. ***\n")
    csv_files = {}
    csv_writers = {}
    
    # Use a try...finally block to ensure files are always closed properly.
    try:
        for drone in drones:
            filename = os.path.join(OUTPUT_DIR, f"{drone.name}_data.csv")
            # Open file in write mode with newline='' to prevent blank rows
            file_handle = open(filename, 'w', newline='')
            writer = csv.writer(file_handle)
            # Write the header row
            writer.writerow(['timestamp', 'all_deltas', 'confidence', 'position'])
            
            csv_files[drone.name] = file_handle
            csv_writers[drone.name] = writer
            info(f"Opened {filename} for data logging.\n")

        stop_event = threading.Event()
        ## Pass the dictionary of csv_writers to the thread.
        fetch_thread = threading.Thread(target=fetch_states, args=(drones, stop_event, csv_writers), daemon=True)
        fetch_thread.start()

        info("\n*** Simulation is running. CSV data is being saved in 'drone_execution_data'. ***\n")
        info("*** Type 'exit' or Ctrl+D in the CLI to quit. ***\n")
        CLI(net)

    finally:
        info("*** Shutting down simulation ***\n")
        if 'stop_event' in locals():
            stop_event.set()
        if 'fetch_thread' in locals():
            fetch_thread.join(timeout=5)
        
        # Close all open CSV files
        for file_handle in csv_files.values():
            file_handle.close()
        info("Closed all data log files.\n")

    info("*** Shutting down simulation ***\n")
    net.stop()

if __name__ == '__main__':
    main()
    # Final cleanup of any lingering Go processes
    os.system(f'sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null')
    info("--- Simulation finished ---\n")
import os
import shutil

from mininet.node import Controller
from mininet.log import setLogLevel, info
from mn_wifi.link import wmediumd, adhoc
from mn_wifi.net import Mininet_wifi
from mn_wifi.cli import CLI
from mn_wifi.telemetry import telemetry 
from mn_wifi.wmediumdConnector import interference

# --- Configuration Constants ---
DRONE_NUMBER = 4
EXEC_PATH = '../drone/bin/drone-linux'  # Path to the compiled Go drone application
OUTPUT_DIR = 'drone_execution_data/'   # Directory for telemetry logs
TCP_PORT = 8080
UDP_PORT = 7000
SPEED = 2
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

    return net

def main():
    """Main execution function."""
    try:
        os.system(f'sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null')
    except:
        pass
    
    setLogLevel('info')
    
    net = setup_topology()
    
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
    CLI(net)

    info("*** Shutting down simulation ***\n")
    net.stop()

if __name__ == '__main__':
    main()
    # Final cleanup of any lingering Go processes
    os.system(f'sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null')
    info("--- Simulation finished ---\n")
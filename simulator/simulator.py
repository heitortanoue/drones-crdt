import time
import os
import shutil

from mininet.node import Controller
from mininet.log import setLogLevel, info
from mn_wifi.link import wmediumd, adhoc
from mn_wifi.net import Mininet_wifi
from mn_wifi.cli import CLI
from mn_wifi.telemetry import telemetry
from mn_wifi.wmediumdConnector import interference

drone_names = []
drone_number = 3
exec_path = '../drone/bin/drone-linux'  # Path to the compiled Go drone application

# Directory to store telemetry data logs
output_dir = 'drone_execution_data/'

def update_telemetry_data_dir(names):
    """Moves generated telemetry files to the output directory"""
    info(f"*** Moving telemetry files to {output_dir}... ***\n")

    os.makedirs(output_dir, exist_ok=True)
    for name in names:
        source_file = f"position-{name}-mn-telemetry.txt"
        destination_file = os.path.join(output_dir, source_file)

        if os.path.exists(source_file):
            shutil.move(source_file, destination_file)
            info(f"-> File {source_file} moved <-\n")

def topology():
    """Creates and runs the network topology for the drone simulation"""
    info("--- Creating a Go drone network with Mininet-WiFi ---\n")

    # Initialize the Mininet-WiFi network with a controller and realistic wireless medium
    net = Mininet_wifi(controller=Controller, link=wmediumd, wmediumd_mode=interference)
    c0 = net.addController('c0')

    # Add a switch to connect all drones, like a bridge network
    info("*** Creating nodes to represent each drone ***\n")

    drones = []
    for i in range (1, drone_number + 1):
        drone_name = f'dr{i}'
        drone_names.append(drone_name)
        mac = f'00:00:00:00:00:0{i}'
        ip = f'10.0.0.{i}/8'
        drone = net.addStation(drone_name,mac=mac,ip=ip,position='10,10,0',txpower=20)
        drones.append(drone)

    info("*** Configuring the signal propagation model ***\n")
    net.setPropagationModel(model="logDistance", exp=4)

    info("*** Configuring network nodes ***\n")
    net.configureNodes()
    i=0
    for drone in drones:
        net.addLink(drone,cls=adhoc,intf=f'{drone_names[i]}-wlan0',ssid='adhocNet',proto='batman_adv',mode='g',channel=5,ht_cap='HT40+')
        i+=1

    info("*** Building the network ***\n")
    net.build()
    net.start()

    info("*** Configuring batman-adv interfaces on each drone ***\n")
    # This loop is crucial for batman-adv to work correctly. It brings up the
    # virtual mesh interface (bat0) and assigns the node's IP address to it,
    # enabling Layer 3 routing over the mesh
    for station in net.stations:
        station.cmd('ip link set up dev bat0')
        station.cmd(f'ip addr add {station.IP()}/8 dev bat0')
        station.cmd('ip route add default dev bat0')

    # Start telemetry to plot drone positions in real-time
    telemetry(nodes=net.stations, single=True, data_type='position')

    info("--- Starting Go applications on drones... ---\n")
    # Each station (drone) runs an instance of the compiled Go application
    # The application handles the high-level logic (gossip, CRDTs, etc.)
    for i, drone in enumerate(net.stations, 1):
        drone_id = f'drone-go-{i}'
        command = (f"{exec_path} -id={drone_id} "
               f"-tcp-port=8080 -udp-port=7000")
        drone.cmd(f'xterm -e "{command}" &')

    # Wait for the Go applications to initialize before starting the CLI
    time.sleep(5)

    info("*** Simulation is running. Use the CLI to interact ***\n")
    CLI(net)

    info("*** Shutting down simulation ***\n")
    net.stop()

    time.sleep(10)  # Ensure all processes have terminated
    update_telemetry_data_dir(drone_names)

if __name__ == '__main__':
    setLogLevel('info')
    # Clean up any leftover Go processes from previous runs to avoid conflicts
    os.system(f'sudo killall -9 {exec_path} &> /dev/null')
    topology()
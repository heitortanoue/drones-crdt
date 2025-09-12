import time
import os
import shutil
from subprocess import Popen

from mininet.node import Controller
from mininet.log import setLogLevel, info
from mn_wifi.link import wmediumd, adhoc
from mn_wifi.net import Mininet_wifi
from mn_wifi.cli import CLI
from mn_wifi.telemetry import telemetry
from mn_wifi.wmediumdConnector import interference

# Global list to track the Go drone application processes
go_drone_processes: list[Popen] = []
drone_names = []
exec_path = './bin/drone-linux'  # Path to the compiled Go drone application

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

def kill_go_processes():
    """Terminates all running Go drone processes."""
    info("*** Terminating Go processes... ***\n")

    global go_drone_processes
    for process in go_drone_processes:
        # Check if the process is still running before trying to terminate it
        if process.poll() is None:
            info(f"-> Stopping Go drone with PID {process.pid}... <-\n")
            process.terminate()
            process.wait()
    go_drone_processes = []
    info("*** All Go drones have been stopped ***\n")

def topology():
    """Creates and runs the network topology for the drone simulation"""
    info("--- Creating a Go drone network with Mininet-WiFi ---\n")

    # Initialize the Mininet-WiFi network with a controller and realistic wireless medium
    net = Mininet_wifi(controller=Controller, link=wmediumd, wmediumd_mode=interference)

    info("*** Creating nodes to represent each drone ***\n")
    dr1 = net.addStation('dr1', mac='00:00:00:00:00:01', ip='10.0.0.1/8', position='30,60,0')
    dr2 = net.addStation('dr2', mac='00:00:00:00:00:02', ip='10.0.0.2/8', position='70,30,0')
    dr3 = net.addStation('dr3', mac='00:00:00:00:00:03', ip='10.0.0.3/8', position='10,20,0')
    c0 = net.addController('c0')

    info("*** Configuring the signal propagation model ***\n")
    net.setPropagationModel(model="logDistance", exp=4.5)

    info("*** Configuring network nodes ***\n")
    net.configureNodes()

    info("*** Adding ad-hoc links for drone-to-drone communication ***\n")
    # The batman_adv protocol creates a mesh network, allowing drones to communicate
    # directly without a central access point, which is ideal for gossip protocols
    net.addLink(dr1, cls=adhoc, intf='dr1-wlan0', ssid='adhocNet', proto='batman_adv', mode='g', channel=5, ht_cap='HT40+')
    net.addLink(dr2, cls=adhoc, intf='dr2-wlan0', ssid='adhocNet', proto='batman_adv', mode='g', channel=5, ht_cap='HT40+')
    net.addLink(dr3, cls=adhoc, intf='dr3-wlan0', ssid='adhocNet', proto='batman_adv', mode='g', channel=5, ht_cap='HT40+')

    info("*** Building the network ***\n")
    net.build()
    c0.start()

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
    for n in net.stations:
        drone_names.append(n.name) # Collect drone names for telemetry file management

    info("--- Starting Go applications on drones... ---\n")
    # Each station (drone) runs an instance of the compiled Go application
    # The application handles the high-level logic (gossip, CRDTs, etc.)
    global go_drone_processes
    for i, drone in enumerate(net.stations, 1):
        drone_id = f'drone-go-{i}'
        udp_port = 7000 + i
        tcp_port = 8080 + i
        process = drone.popen(f'{exec_path} -id={drone_id} -udp-port={udp_port} -tcp-port={tcp_port}')
        go_drone_processes.append(process)
        info(f"*** Drone {drone_id} started with PID: {process.pid} (UDP:{udp_port}, TCP:{tcp_port}) ***\n")

    # Wait for the Go applications to initialize before starting the CLI
    time.sleep(5)

    info("*** Simulation is running. Use the CLI to interact ***\n")
    CLI(net)

    info("*** Shutting down simulation ***\n")
    kill_go_processes()
    net.stop()

    time.sleep(5)  # Ensure all processes have terminated
    update_telemetry_data_dir(drone_names)

if __name__ == '__main__':
    setLogLevel('info')
    # Clean up any leftover Go processes from previous runs to avoid conflicts
    os.system(f'sudo killall -9 {exec_path} &> /dev/null')
    topology()
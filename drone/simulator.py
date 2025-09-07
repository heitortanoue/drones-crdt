#!/usr/bin/python

import time
import os
import curses
import shutil

from mininet.log import setLogLevel, info
from mn_wifi.link import wmediumd, adhoc
from mn_wifi.net import Mininet_wifi
from mn_wifi.telemetry import telemetry
from mn_wifi.wmediumdConnector import interference

# Global list to keep track of Go drone processes
go_drone_processes = []
drone_names = []

# Directory to save telemetry data
output_dir = 'drone_execution_data/'

def keyboard_control(net):
    """
    Allows manual control of drones using keyboard inputs
    """
    drones = net.stations
    selected_drone_index = 0
    move_step = 2  # Step size for each movement

    stdscr = curses.initscr()
    curses.cbreak()
    stdscr.keypad(True)
    stdscr.nodelay(True)
    curses.noecho()

    info("*** Starting manual control of drones ***\n")
    info("Use arrow keys to move, TAB to switch drones, 'q' to quit.\n")

    try:
        while True:
            drone = drones[selected_drone_index]
            
            stdscr.clear()
            stdscr.addstr(0, 0, "Drone Control Activated")
            stdscr.addstr(1, 0, "Press 'q' to quit and stop the simulation.")
            stdscr.addstr(3, 0, f"--> Selected Drone: {drone.name} [IP: {drone.IP()}]")

            for i, d in enumerate(drones):
                current_pos_str = d.position if hasattr(d, 'position') else "N/A"
                if i != selected_drone_index:
                    stdscr.addstr(4 + i, 0, f"    Drone: {d.name} [Position: {current_pos_str}]")

            current_pos = drone.position
            stdscr.addstr(4 + selected_drone_index, 0, f"--> Drone: {drone.name} [Position: {current_pos}] <--")
            stdscr.refresh()

            key = stdscr.getch()

            if key == ord('q'):
                break

            elif key == ord('\t'):
                selected_drone_index = (selected_drone_index + 1) % len(drones)
                time.sleep(0.1)
            
            elif key in [curses.KEY_UP, curses.KEY_DOWN, curses.KEY_LEFT, curses.KEY_RIGHT]:
                x, y, z = float(current_pos[0]), float(current_pos[1]), float(current_pos[2])
                
                if key == curses.KEY_UP:
                    y += move_step
                elif key == curses.KEY_DOWN:
                    y -= move_step
                elif key == curses.KEY_LEFT:
                    x -= move_step
                elif key == curses.KEY_RIGHT:
                    x += move_step
                
                drone.position = (x, y, z)
            
            time.sleep(0.1)
    finally:
        curses.nocbreak()
        stdscr.keypad(False)
        curses.echo()
        curses.endwin()
        info("\n*** Manual control ended. ***\n")

def topology():
    info("--- Creating a network of multiple Go drones with Mininet-WiFi ---\n")
    net = Mininet_wifi(link=wmediumd, wmediumd_mode=interference)

    info("*** Creating nodes representing each of the drones ***\n")
    dr1 = net.addStation('dr1', mac='00:00:00:00:00:01', ip='10.0.0.1/8',
                         position='30,60,0')
    dr2 = net.addStation('dr2', mac='00:00:00:00:00:02', ip='10.0.0.2/8',
                         position='70,30,0')
    dr3 = net.addStation('dr3', mac='00:00:00:00:00:03', ip='10.0.0.3/8',
                         position='10,20,0')

    net.setPropagationModel(model="logDistance", exp=4.5)

    info("*** Configuring nodes ***\n")
    net.configureNodes()

    # Adding ad-hoc links for all drones
    net.addLink(dr1, cls=adhoc, intf='dr1-wlan0',
                ssid='adhocNet', proto='batman_adv',
                mode='g', channel=5, ht_cap='HT40+')

    net.addLink(dr2, cls=adhoc, intf='dr2-wlan0',
                ssid='adhocNet', proto='batman_adv',
                mode='g', channel=5, ht_cap='HT40+')

    net.addLink(dr3, cls=adhoc, intf='dr3-wlan0',
                ssid='adhocNet', proto='batman_adv',
                mode='g', channel=5, ht_cap='HT40+')


    info("*** Starting network ***\n\n")
    net.build()

    nodes = net.stations
    telemetry(nodes=nodes, single=True, data_type='position')

    for n in net.stations:
        drone_names.append(n.name)

    info("*** Starting socket server ***\n")
    net.socketServer(ip='127.0.0.1', port=12345)

    info("--- Starting Go drones... ---\n")
    global go_drone_processes
    # The Go code must be compiled first and the executable must be in the path
    for i, drone in enumerate(net.stations, 1):
        drone_id = f'drone-go-{i}'
        udp_port = 7000 + i
        tcp_port = 8080 + i
        process = drone.popen(f'./main -id={drone_id} -udp-port={udp_port} -tcp-port={tcp_port}')
        go_drone_processes.append(process)
        info(f"*** Drone {drone_id} started with PID: {process.pid} (UDP:{udp_port}, TCP:{tcp_port}) ***\n")

    time.sleep(5)  # Wait for Go drones to initialize

    info("*** Starting manual control. Open the telemetry view in another terminal ***\n")
    keyboard_control(net)

    info("*** Stopping network ***\n")
    kill_process()
    net.stop()


def kill_process():
    info("Killing processes...\n")

    global go_drone_processes
    for process in go_drone_processes:
        if process.poll() is None:
            info(f"*** Stopping Go drone with PID {process.pid}... ***\n")
            process.terminate()
            process.wait()
    go_drone_processes = []
    info("*** All Go drones have been stopped. ***\n")

if __name__ == '__main__':
    setLogLevel('info')
    kill_process()
    topology()
import json
from datetime import datetime
from typing import List, Set

from config import (
    ATTENUATION,
    DRONE_HEIGHT,
    DRONE_NAMES,
    DRONE_RANGE,
    PROPAGATION_MODEL,
    SPEED,
    TCP_PORT,
    X_MAX,
    Y_MAX,
    duration,
)
from mininet.log import info
from mininet.node import Controller
from mn_wifi.link import adhoc, wmediumd
from mn_wifi.net import Mininet_wifi
from mn_wifi.wmediumdConnector import interference


def setup_topology():
    """Creates and configures the network topology for the drone simulation."""
    info("--- Creating a Go drone network with Mininet-WiFi ---\n")
    drones = []
    net = Mininet_wifi(link=wmediumd, wmediumd_mode=interference)
    net.addController("c0")

    info("*** Creating drone nodes ***\n")
    kwargs = {}
    kwargs["height"] = DRONE_HEIGHT
    for i, name in enumerate(DRONE_NAMES, 1):
        # Generate MAC address properly for any number of drones
        mac = f"00:00:00:00:{(i >> 8):02x}:{(i & 0xff):02x}"
        # Generate IP address for up to 65534 drones (255.254 in class A network)
        ip = f"10.{(i >> 8) & 0xff}.{i & 0xff}.0/8"

        drone = net.addStation(
            name,
            mac=mac,
            ip=ip,
            range=DRONE_RANGE,
            min_x=0,
            max_x=X_MAX,
            min_y=0,
            max_y=Y_MAX,
            min_v=0.6 * SPEED,
            max_v=SPEED,
            **kwargs,
        )
        drones.append(drone)

    info("*** Configuring the signal propagation model ***\n")
    net.setPropagationModel(model=PROPAGATION_MODEL, exp=ATTENUATION)
    info("*** Configuring network nodes ***\n")
    net.configureNodes()
    net.plotGraph()
    net.plotEnergyMonitor()
    net.setMobilityModel(
        time=0, model="GaussMarkov", max_x=X_MAX, max_y=Y_MAX, velocity=SPEED, seed=20
    )

    info("*** Adding ad-hoc links to drones ***\n")
    kwargs["proto"] = "batman_adv"
    for drone in drones:
        net.addLink(
            drone,
            cls=adhoc,
            intf=f"{drone.name}-wlan0",
            ssid="adhocNet",
            mode="g",
            channel=5,
            ht_cap="HT40+",
            **kwargs,
        )

    return net, drones


def send_drone_location(drone):
    """Sends the current location of the drone to its Go application."""
    position = drone.position
    command = f"""curl -X POST http://{drone.IP()}:{TCP_PORT}/position \
    -H "Content-Type: application/json" \
    -d '{{"x": {int(position[0])}, "y": {int(position[1])}}}'"""
    response = drone.cmd(command).strip()


def send_locations(drones, stop_event):
    """Sends the locations of all drones periodically."""
    while not stop_event.is_set():
        stop_event.wait(duration)
        if stop_event.is_set():
            break
        for drone in drones:
            send_drone_location(drone)


def fetch_stats(drone):
    command = f"curl -s --max-time 5 http://{drone.IP()}:{TCP_PORT}/stats"
    response_str = drone.cmd(command).strip()

    ## Parse the JSON response and log the specific fields.
    try:
        data = json.loads(response_str)
        return data

    except json.JSONDecodeError as e:
        # Handle cases where the response is not valid JSON
        info(f"-> ERROR for {drone.name}: Could not parse JSON response <-\n")
        info(f"   Problematic response: {response_str}\n")


def fetch_state(drone):
    command = f"curl -s --max-time 5 http://{drone.IP()}:{TCP_PORT}/state"
    response_str = drone.cmd(command).strip()

    ## Parse the JSON response and log the specific fields.
    try:
        data = json.loads(response_str)
        # Extract data, assuming the first entry in latest_readings is the relevant one
        reading_data = list(data["latest_readings"].values())[0]
        timestamp_ms = reading_data["timestamp"]
        confidence = reading_data["confidence"]
        all_deltas = data["all_deltas"]
        return timestamp_ms, confidence, all_deltas

    except json.JSONDecodeError as e:
        # Handle cases where the response is not valid JSON
        info(f"-> ERROR for {drone.name}: Could not parse JSON response <-\n")
        info(f"   Problematic response: {response_str}\n")


def fetch_states(drones, stop_event, csv_writers):
    """Fetches and logs the state of the drones periodically."""
    repetitions = 0
    convergence = 0.0
    while not stop_event.is_set():
        stop_event.wait(duration)
        if stop_event.is_set():
            break

        drone_delta_sets: List[Set[str]] = [set() for _ in drones]
        for i, drone in enumerate(drones):
            position = drone.position
            writer = csv_writers[drone.name]
            timestamp_ms, confidence, all_deltas = fetch_state(drone)
            for delta in all_deltas:
                drone_delta_sets[i].add(json.dumps(delta))

            # Format the timestamp from milliseconds to a readable string
            formatted_timestamp = datetime.fromtimestamp(
                timestamp_ms / 1000
            ).isoformat()

            # Convert all_deltas list to a compact JSON string for storage in a single CSV cell
            deltas_str = json.dumps(all_deltas)

            # Write the parsed data to the CSV file
            writer.writerow(
                [
                    formatted_timestamp,
                    deltas_str,
                    confidence,
                    position,
                    repetitions,
                    convergence,
                ]
            )

        # Check for convergence
        repetitions += 1
        convergence = convergence_index(drone_delta_sets)
        info(f"--- Repetition {repetitions}: Convergence = {convergence:.4f} ---\n")
        if convergence == 1.0:
            info("-> All drones have converged! <-\n")
            info(f"-> Convergence achieved after {repetitions * duration} seconds <-\n")


def jaccard_index(set1: Set, set2: Set) -> float:
    """Calculates the Jaccard index between two sets."""
    if not set1 and not set2:
        return 1.0  # both empty → fully converged
    inter = len(set1 & set2)
    uni = len(set1 | set2)
    return inter / uni


def convergence_index(replicas: List[Set]) -> float:
    """
    Calculates the average convergence index between multiple CRDT replicas (based on Jaccard).
    Returns a value between 0 and 1.
    """
    if len(replicas) < 2:
        return 1.0  # only one replica → total convergence

    scores = []
    n = len(replicas)
    for i in range(n):
        for j in range(i + 1, n):
            scores.append(jaccard_index(replicas[i], replicas[j]))
    return sum(scores) / len(scores)

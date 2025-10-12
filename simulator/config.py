# Drone setup configuration file
# Adjust parameters here to change the simulation setup
DRONE_NUMBER = 3
DRONE_SPEED = 20  # Maximum speed of each drone in m/s
DRONE_RANGE = 300  # Communication range of each drone in meters
DRONE_HEIGHT = 50  # Height of each drone in meters

# The propagation model is set as 'logDistance'
# With the attenuation exponent you can control the signal strength
# According to the environment you are simulating
# For this simulation, we are considering an outdoors rural environment
PROPAGATION_MODEL = "logDistance"
ATTENUATION = 4.5  # Attenuation exponent for the propagation model

# Simulator configuration
X_MAX, Y_MAX = 2500, 2500  # Size of the simulation area
SIMULATION_MULTIPLIER = 1  # Speed multiplier for the simulation time
FETCH_INTERVAL = 8  # Interval in seconds to fetch drone states
DELTA_PUSH_INTERVAL = 1  # Interval in seconds to push deltas to neighbors
ANTI_ENTROPY_INTERVAL = 2  # Interval in seconds for anti-entrop
BIND_ADDR = "0.0.0.0"  # Address to bind the drone application
TTL = 4  # Time-to-live for gossip messages
FANOUT = 3  # Number of neighbors to gossip with

EXEC_PATH = "../drone/bin/drone-linux"  # Path to the compiled Go drone application
OUTPUT_DIR = "drone_execution_data/"  # Directory for telemetry logs

# ------------------- Configuration Constants -------------------
# DO NOT MODIFY BELOW THIS LINE UNLESS YOU KNOW WHAT YOU ARE DOING
SPEED = DRONE_SPEED * SIMULATION_MULTIPLIER
TCP_PORT = 8080
UDP_PORT = 7000
DRONE_NAMES = [f"dr{i}" for i in range(1, DRONE_NUMBER + 1)]
DRONE_IPs = {
    f"http://10.{(i >> 8) & 0xff}.{i & 0xff}.0:{TCP_PORT}": name
    for i, name in enumerate(DRONE_NAMES, 1)
}
duration = (
    FETCH_INTERVAL / SIMULATION_MULTIPLIER
)  # fetch state every 'duration' seconds, considering the speed multiplier
delta_push_interval = (
    DELTA_PUSH_INTERVAL / SIMULATION_MULTIPLIER
)  # push deltas every 'delta_push_interval' seconds, considering the speed multiplier
anti_entropy_interval = (
    ANTI_ENTROPY_INTERVAL / SIMULATION_MULTIPLIER
)  # anti-entropy every 'anti_entropy_interval' seconds, considering the speed multiplier
ttl = (
    TTL / SIMULATION_MULTIPLIER
)  # TTL every 'ttl' seconds, considering the speed multiplier
# ---------------------------------------------------------------

# Drone setup configuration file
# Adjust parameters here to change the simulation setup
DRONE_NUMBER = 10
DRONE_SPEED = 20  # Maximum speed of each drone in m/s
DRONE_RANGE = 300  # Communication range of each drone in meters
DRONE_HEIGHT = 50  # Height of each drone in meters

# The propagation model is set as 'logDistance'
# With the attenuation exponent you can control the signal strength
# According to the environment you are simulating
# For this simulation, we are considering an outdoors rural environment
PROPAGATION_MODEL = "logDistance"  # Propagation model to use
ATTENUATION = 4.5  # Attenuation exponent for the propagation model
MOBILITY_MODEL = "TruncatedLevyWalk"  # Mobility model for the drones

# Simulator configuration
X_MAX, Y_MAX = 2500, 2500  # Size of the simulation area
SIMULATION_MULTIPLIER = 5  # Speed multiplier for the simulation time
FETCH_INTERVAL = 10  # Interval in seconds to fetch drone states
DELTA_PUSH_INTERVAL = 3  # Interval in seconds to push deltas to neighbors
ANTI_ENTROPY_INTERVAL = 60  # Interval in seconds for anti-entropy
BIND_ADDR = "0.0.0.0"  # Address to bind the drone application
TTL = 4  # Time-to-live for gossip messages
FANOUT = 3  # Number of neighbors to gossip with

# Sensor configuration
SAMPLE_INTERVAL_SECONDS = 10  # Interval in seconds between sensor samples
CONFIDENCE_THRESHOLD = (
    50.0  # Minimum confidence threshold (0-100) to accept fire detection
)

# Network timeouts
NEIGHBOR_TIMEOUT_SEC = 5  # Neighbor timeout in seconds
TRANSMITTER_TIMEOUT_SEC = 3  # Transmitter timeout in seconds

# Control protocol configuration
HELLO_INTERVAL_MS = 1000  # Base interval for hello messages in milliseconds
HELLO_JITTER_MS = 200  # Random jitter added to hello interval in milliseconds

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

delta_push_interval = (
    DELTA_PUSH_INTERVAL / SIMULATION_MULTIPLIER
)  # push deltas every 'delta_push_interval' seconds, considering the speed multiplier
anti_entropy_interval = (
    ANTI_ENTROPY_INTERVAL / SIMULATION_MULTIPLIER
)  # anti-entropy every 'anti_entropy_interval' seconds, considering the speed multiplier
sample_interval_sec = (
    SAMPLE_INTERVAL_SECONDS / SIMULATION_MULTIPLIER
)
neighbor_timeout_sec = NEIGHBOR_TIMEOUT_SEC / SIMULATION_MULTIPLIER
transmitter_timeout_sec = TRANSMITTER_TIMEOUT_SEC / SIMULATION_MULTIPLIER
hello_interval_ms = HELLO_INTERVAL_MS / SIMULATION_MULTIPLIER
hello_jitter_ms = HELLO_JITTER_MS / SIMULATION_MULTIPLIER
confidence_threshold = CONFIDENCE_THRESHOLD
# ---------------------------------------------------------------

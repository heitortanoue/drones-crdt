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
PROPAGATION_MODEL = 'logDistance'
ATTENUATION = 4.5  # Attenuation exponent for the propagation model

# Simulator configuration
X_MAX, Y_MAX = 2500, 2500  # Size of the simulation area
SIMULATION_MULTIPLIER = 1  # Speed multiplier for the simulation time
FETCH_INTERVAL = 8  # Interval in seconds to fetch drone states

EXEC_PATH = '../drone/bin/drone-linux'  # Path to the compiled Go drone application
OUTPUT_DIR = 'drone_execution_data/'   # Directory for telemetry logs

# ------------------- Configuration Constants -------------------
# DO NOT MODIFY BELOW THIS LINE UNLESS YOU KNOW WHAT YOU ARE DOING
SPEED = DRONE_SPEED * SIMULATION_MULTIPLIER 
TCP_PORT = 8080
UDP_PORT = 7000
DRONE_NAMES = [f'dr{i}' for i in range(1, DRONE_NUMBER + 1)]
DRONE_IPs = {
    f'http://192.168.123.{i}:{TCP_PORT}': name
    for i, name in enumerate(DRONE_NAMES, 1)
}
duration = FETCH_INTERVAL / SIMULATION_MULTIPLIER  # fetch state every 'duration' seconds, considering the speed multiplier
# ---------------------------------------------------------------
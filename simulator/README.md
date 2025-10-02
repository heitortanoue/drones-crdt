# Drone Swarm Simulator

This project simulates the behavior of a swarm of drones using the Mininet-WiFi network emulator. It is designed to study drone communication, data collection, and swarm convergence under various configurations.

## Prerequisites

* **Operating System:** An Ubuntu-based Linux distribution is required.

## Installation

This simulator depends on Mininet-WiFi. Follow these steps to install it and its dependencies.

1.  **Install Dependencies**
    First, install `ansible` and `aptitude`, which are required by the Mininet-WiFi installation script.
    ```bash
    sudo apt-get update
    sudo apt-get install -y ansible aptitude
    ```

2.  **Clone the Containernet/Mininet-WiFi Repository**
    Clone the [repository from GitHub](https://github.com/ramonfontes/containernet), which contains the necessary files for Mininet-WiFi.
    ```bash
    git clone https://github.com/ramonfontes/containernet
    ```

3.  **Run the Installation Script**
    Navigate into the cloned repository directory and run the installer. This script will install Mininet-WiFi and all its related components.
    ```bash
    cd containernet
    sudo util/install.sh
    ```

4.  **Install the Python Package**
    After the script finishes, install the package using `setup.py`.
    ```bash
    sudo python setup.py install
    ```

## Running the Simulation

Once the installation is complete, you can run the simulator using the main script. You must run it with `sudo` privileges because Mininet-WiFi requires them to create and manage virtual network interfaces.

```bash
sudo python simulator.py
```

## Configuration

You can easily modify the simulation parameters by editing the variables directly in the `simulator.py` file.

* `DRONE_NUMBER`: This variable sets the total number of drones in the simulation.
    ```python
    # example in simulator.py
    DRONE_NUMBER = 10
    ```

* `DRONE_RANGE`: This variable defines the communication range for each drone's station.
    ```python
    # example in simulator.py
    DRONE_RANGE = 50
    ```
    **Note:** If you set a very high `DRONE_RANGE`, you may also need to increase the `txpower` (transmission power) of the stations within the script to ensure the signal strength is sufficient to cover that distance.

## Output

At the end of each simulation run, a set of `.csv` files is generated in a new directory called `drone_execution_data` (the name can be updated by changing the `OUTPUT_DIR` variable), one for each drone (e.g., `drone1_data.csv`, `drone2_data.csv`, etc.).

Each CSV file contains the data collected by that specific drone and includes the following columns:

* **timestamp**: The simulation time when the data point was recorded.
* **position**: The (x, y, z) coordinates of the drone.
* **deltas**: The change in position or other measured values since the last timestamp.
* **confidence**: A metric indicating the drone's confidence in its current state or measurement.
* **convergence**: A flag or value indicating if the swarm has reached a convergence state.
* **repetition**: An identifier for the current convergence repetition cycle.
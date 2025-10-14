import tkinter as tk
from tkinter import ttk
from mininet.log import info
from datetime import datetime

from drone_utils import fetch_state, fetch_stats
from config import (
    DRONE_NAMES, DRONE_IPs
)

class DroneControlPanel:
    def __init__(self, root, drone_list):
        """Basic UI for monitoring drones."""
        self.root = root
        self.root.title("Drone Monitoring Panel")
        self.root.geometry("600x400") # Initial size of the window
        self.drone_list = drone_list

        # Widget style
        self.style = ttk.Style()
        self.style.theme_use('clam') # 'clam', 'alt', 'default', 'classic'

        # --- Widgets Creation ---

        # Main frame to organize content
        main_frame = ttk.Frame(self.root, padding="20")
        main_frame.pack(expand=True, fill='both')

        # 1. Label and Drone Selection Menu
        ttk.Label(
            main_frame,
            text="Select Drone:",
            font=("Helvetica", 12)
        ).grid(row=0, column=0, padx=5, pady=10, sticky='w')

        self.drone_selector = ttk.Combobox(
            main_frame,
            values=DRONE_NAMES,
            state='readonly',
            font=("Helvetica", 11)
        )
        self.drone_selector.grid(row=0, column=1, padx=5, pady=10, sticky='ew')
        self.drone_selector.current(0)  # Default to the first drone

        # 2. Button Frame
        button_frame = ttk.Frame(main_frame)
        button_frame.grid(row=1, column=0, columnspan=2, pady=15)

        # Deltas button
        self.position_button = ttk.Button(
            button_frame,
            text="View Drone Deltas",
            command=self.show_deltas,
        )
        self.position_button.pack(side='left', padx=10)

        # Location button
        self.location_button = ttk.Button(
            button_frame,
            text="View Location",
            command=self.show_location_info,
        )
        self.location_button.pack(side='left', padx=10)

        # Stats button
        self.stats_button = ttk.Button(
            button_frame,
            text="View Stats",
            command=self.show_stats_info
        )
        self.stats_button.pack(side='left', padx=10)

        # 3. Label to display information
        self.info_label = ttk.Label(
            main_frame,
            text="Select a drone and click a button.",
            font=("Helvetica", 12),
            foreground="gray",
        )
        self.info_label.grid(row=2, column=0, columnspan=2, pady=20)

        # Configures the grid to expand correctly with the window
        main_frame.columnconfigure(1, weight=1)

    # --- Functions called by the buttons ---
    def get_selected_drone(self):
        """Returns the name of the selected drone in the menu."""
        for drone in self.drone_list:
            if drone.name == DRONE_NAMES[self.drone_selector.current()]:
                return drone

    def show_location_info(self):
        """Shows the current location of the selected drone."""
        drone = self.get_selected_drone()
        if not drone:
            self.info_label.config(text="Error: No drone selected.", foreground="red")
            return
        self.info_label.config(text=f"Position of {drone.name}\n X: {drone.position[0]}, Y: {drone.position[1]}", foreground="blue")

    def show_deltas(self):
        """Shows information about the deltas."""
        drone = self.get_selected_drone()
        if not drone:
            self.info_label.config(text="Error: No drone selected.", foreground="red")
            return
        timestamp_ms, confidence, all_deltas = fetch_state(drone)
        self.info_label.config(text=f"Deltas of {drone.name}\nTimestamp: {datetime.fromtimestamp(timestamp_ms / 1000).isoformat()}\nConfidence: {confidence}\nDeltas: {all_deltas}", foreground="blue")

    def show_stats_info(self):
        """Shows the stats of the selected drone."""
        stats, drone_name = self.get_stats()
        if not stats:
            self.info_label.config(text="Error: Could not fetch stats.", foreground="red")
            return
        neighbour_text = f'Currently {drone_name} has no neighbours in range.'
        neighbours = self.get_neighbours_names(stats, drone_name)
        if neighbours:
            neighbour_text = f"Currently {drone_name} has neighbours in range: {', '.join(neighbours)}"
        self.info_label.config(
            text=f"{neighbour_text}\n \
            Is running?: {stats['control']['running']}\n \
            Dissemination values\n \
            Anti entropy count: {stats['dissemination']['anti_entropy_count']}\n \
            Anti entropy interval (in seconds): {stats['dissemination']['anti_entropy_interval_sec']}\n \
            Default ttl: {stats['dissemination']['default_ttl']}\n \
            Delta push interval (in seconds): {stats['dissemination']['delta_push_interval_sec']}\n \
            Sent Count: {stats['dissemination']['sent_count']}\n \
            Received Count: {stats['dissemination']['received_count']}\n \
            Dropped Count: {stats['dissemination']['dropped_count']}\n \
            Fanout: {stats['dissemination']['fanout']}\n \
            Sensor System\n \
            Active fires: {stats['sensor_system']['generator']['active_fires']}\n \
            Reading count: {stats['sensor_system']['reading_count']}\n \
            Up time: {stats['uptime']}", foreground="blue")

    def get_stats(self):
        """Fetches and returns the stats of the selected drone."""
        drone = self.get_selected_drone()
        if not drone:
            self.info_label.config(text="Error: No drone selected.", foreground="red")
            return
        return fetch_stats(drone), drone.name

    def get_neighbours_names(self, stats, drone_name):
        """Fetches and returns the neighbours of the selected drone."""
        neighbour_drones = []
        neighbours_url = stats['network']['neighbor_urls']
        if not neighbours_url:
            self.info_label.config(text=f"{drone_name} has no neighbours in range.", foreground="blue")
            return []

        for drone_url in neighbours_url:
            drone = DRONE_IPs.get(drone_url)
            if drone != drone_name and drone is not None and drone not in neighbour_drones:
                neighbour_drones.append(drone)
                
        return neighbour_drones

def setup_UI(drones):
    """Creates and configures the UI for the drone simulation."""
    info("--- Creating the drone UI ---\n")
    root = tk.Tk()
    app = DroneControlPanel(root, drone_list=drones)
    root.mainloop()

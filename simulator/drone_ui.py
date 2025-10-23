import json
import tkinter as tk
from datetime import datetime
from tkinter import ttk

from config import DRONE_NAMES, DRONE_IPs
from drone_utils import fetch_state, fetch_stats
from mininet.log import info


class DroneControlPanel:
    def __init__(self, root, drone_list):
        """Basic UI for monitoring drones."""
        self.root = root
        self.root.title("Drone Monitoring Panel")
        self.root.geometry("600x400")  # Initial size of the window
        self.drone_list = drone_list

        # Widget style
        self.style = ttk.Style()
        self.style.theme_use("clam")  # 'clam', 'alt', 'default', 'classic'

        # --- Widgets Creation ---

        # Main frame to organize content
        main_frame = ttk.Frame(self.root, padding="20")
        main_frame.pack(expand=True, fill="both")

        # 1. Label and Drone Selection Menu
        ttk.Label(main_frame, text="Select Drone:", font=("Helvetica", 12)).grid(
            row=0, column=0, padx=5, pady=10, sticky="w"
        )

        self.drone_selector = ttk.Combobox(
            main_frame, values=DRONE_NAMES, state="readonly", font=("Helvetica", 11)
        )
        self.drone_selector.grid(row=0, column=1, padx=5, pady=10, sticky="ew")
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
        self.position_button.pack(side="left", padx=10)

        # Location button
        self.location_button = ttk.Button(
            button_frame,
            text="View Location",
            command=self.show_location_info,
        )
        self.location_button.pack(side="left", padx=10)

        # Stats button
        self.stats_button = ttk.Button(
            button_frame, text="View Stats", command=self.show_stats_info
        )
        self.stats_button.pack(side="left", padx=10)

        # 3. Scrollable Text widget for JSON display
        text_frame = ttk.Frame(main_frame)
        text_frame.grid(row=2, column=0, columnspan=2, pady=10, sticky="nsew")

        # Scrollbar
        scrollbar = ttk.Scrollbar(text_frame)
        scrollbar.pack(side="right", fill="y")

        # Text widget
        self.info_text = tk.Text(
            text_frame,
            wrap="word",
            font=("Courier", 10),
            bg="#1e1e1e",
            fg="#d4d4d4",
            insertbackground="white",
            yscrollcommand=scrollbar.set,
            relief="solid",
            borderwidth=1,
        )
        self.info_text.pack(side="left", fill="both", expand=True)
        scrollbar.config(command=self.info_text.yview)

        # Configure syntax highlighting tags
        self.info_text.tag_configure("key", foreground="#9cdcfe")
        self.info_text.tag_configure("string", foreground="#ce9178")
        self.info_text.tag_configure("number", foreground="#b5cea8")
        self.info_text.tag_configure("boolean", foreground="#569cd6")
        self.info_text.tag_configure("null", foreground="#569cd6")
        self.info_text.tag_configure("bracket", foreground="#ffd700")

        # Initial message
        self.display_text("Select a drone and click a button.", color="gray")

        # Configures the grid to expand correctly with the window
        main_frame.columnconfigure(1, weight=1)
        main_frame.rowconfigure(2, weight=1)

    # --- Functions called by the buttons ---
    def get_selected_drone(self):
        """Returns the name of the selected drone in the menu."""
        for drone in self.drone_list:
            if drone.name == DRONE_NAMES[self.drone_selector.current()]:
                return drone

    def display_text(self, text, color="white"):
        """Display plain text in the text widget."""
        self.info_text.config(state="normal")
        self.info_text.delete("1.0", "end")
        self.info_text.insert("1.0", text)
        self.info_text.tag_add("plain", "1.0", "end")
        self.info_text.tag_configure("plain", foreground=color)
        self.info_text.config(state="disabled")

    def display_json(self, json_text):
        """Display JSON with syntax highlighting."""
        self.info_text.config(state="normal")
        self.info_text.delete("1.0", "end")

        # Simple regex-like approach for syntax highlighting
        import re

        lines = json_text.split("\n")
        for line in lines:
            # Find keys (before :)
            key_pattern = r'"([^"]+)"\s*:'
            last_end = 0
            for match in re.finditer(key_pattern, line):
                # Insert text before the match
                if match.start() > last_end:
                    self.info_text.insert("end", line[last_end : match.start()])
                # Insert the key with highlighting
                self.info_text.insert("end", match.group(0), "key")
                last_end = match.end()

            # Insert remaining part of line
            remaining = line[last_end:]

            # Highlight strings (values)
            string_pattern = r':\s*"([^"]*)"'
            parts = re.split(r'("(?:[^"\\]|\\.)*")', remaining)
            for i, part in enumerate(parts):
                if i % 2 == 1 and part.startswith('"'):  # String value
                    self.info_text.insert("end", part, "string")
                else:
                    # Highlight numbers, booleans, null
                    number_pattern = r"\b(-?\d+\.?\d*)\b"
                    boolean_pattern = r"\b(true|false)\b"
                    null_pattern = r"\b(null)\b"
                    bracket_pattern = r"([{}\[\],])"

                    temp = part
                    temp = re.sub(
                        number_pattern, lambda m: f"<NUMBER>{m.group(0)}</NUMBER>", temp
                    )
                    temp = re.sub(
                        boolean_pattern, lambda m: f"<BOOL>{m.group(0)}</BOOL>", temp
                    )
                    temp = re.sub(
                        null_pattern, lambda m: f"<NULL>{m.group(0)}</NULL>", temp
                    )
                    temp = re.sub(
                        bracket_pattern,
                        lambda m: f"<BRACKET>{m.group(0)}</BRACKET>",
                        temp,
                    )

                    # Insert with tags
                    segments = re.split(r"<(NUMBER|BOOL|NULL|BRACKET)>(.*?)</\1>", temp)
                    for j, seg in enumerate(segments):
                        if j % 3 == 0:
                            self.info_text.insert("end", seg)
                        elif j % 3 == 2:
                            tag_name = segments[j - 1].lower()
                            self.info_text.insert("end", seg, tag_name)

            self.info_text.insert("end", "\n")

        self.info_text.config(state="disabled")

    def show_location_info(self):
        """Shows the current location of the selected drone."""
        drone = self.get_selected_drone()
        if not drone:
            self.display_text("Error: No drone selected.", color="red")
            return

        location_data = {
            "drone": drone.name,
            "position": {"x": int(drone.position[0]), "y": int(drone.position[1])},
        }
        self.display_json(json.dumps(location_data, indent=2))

    def show_deltas(self):
        """Shows information about the deltas - fetches full /state response."""
        from config import TCP_PORT

        drone = self.get_selected_drone()
        if not drone:
            self.display_text("Error: No drone selected.", color="red")
            return

        # Fetch the raw state response
        command = f"curl -s --max-time 5 http://{drone.IP()}:{TCP_PORT}/state"
        response_str = drone.cmd(command).strip()

        try:
            # Parse and re-format the JSON
            data = json.loads(response_str)
            self.display_json(json.dumps(data, indent=2))
        except json.JSONDecodeError:
            self.display_text(
                f"Error: Could not parse JSON response\n\n{response_str}", color="red"
            )

    def show_stats_info(self):
        """Shows the stats of the selected drone - fetches full /stats response."""
        from config import TCP_PORT

        drone = self.get_selected_drone()
        if not drone:
            self.display_text("Error: No drone selected.", color="red")
            return

        # Fetch the raw stats response
        command = f"curl -s --max-time 5 http://{drone.IP()}:{TCP_PORT}/stats"
        response_str = drone.cmd(command).strip()

        try:
            # Parse and re-format the JSON
            data = json.loads(response_str)
            self.display_json(json.dumps(data, indent=2))
        except json.JSONDecodeError:
            self.display_text(
                f"Error: Could not parse JSON response\n\n{response_str}", color="red"
            )


def setup_UI(drones):
    """Creates and configures the UI for the drone simulation."""
    info("--- Creating the drone UI ---\n")
    root = tk.Tk()
    app = DroneControlPanel(root, drone_list=drones)
    root.mainloop()

import os
import threading
import json
import csv
import tkinter as tk
from tkinter import ttk
from mininet.node import Controller
from mininet.log import setLogLevel, info
from mn_wifi.link import wmediumd, adhoc
from mn_wifi.net import Mininet_wifi
from mn_wifi.cli import CLI
from mn_wifi.wmediumdConnector import interference
from datetime import datetime
from typing import List, Set

# --- Configuration Constants ---
DRONE_NUMBER = 4
DRONE_RANGE = 30  # Communication range of each drone in meters

EXEC_PATH = '../drone/bin/drone-linux'  # Path to the compiled Go drone application
OUTPUT_DIR = 'drone_execution_data/'   # Directory for telemetry logs
TCP_PORT = 8080
UDP_PORT = 7000
SPEED = 2
DRONE_NAMES = [f'dr{i}' for i in range(1, DRONE_NUMBER + 1)]
DRONE_IPs = []
duration = 10  # Duration to run the simulation before fetching data

class DroneControlPanel:
    def __init__(self, root, drone_list):
        """Construtor da nossa classe de interface gráfica."""
        self.root = root
        self.root.title("Painel de Controle de Drones")
        self.root.geometry("500x250") # Define o tamanho inicial da janela
        self.drone_list = drone_list

        # Define um estilo para os widgets
        self.style = ttk.Style()
        self.style.theme_use('clam') # 'clam', 'alt', 'default', 'classic'

        # --- Criação dos Widgets (Componentes da tela) ---

        # Frame principal para organizar o conteúdo
        main_frame = ttk.Frame(self.root, padding="20")
        main_frame.pack(expand=True, fill='both')

        # 1. Label e Menu de Seleção de Drone
        ttk.Label(main_frame, text="Selecione o Drone:", font=("Helvetica", 12)).grid(row=0, column=0, padx=5, pady=10, sticky='w')

        self.drone_selector = ttk.Combobox(
            main_frame, 
            values=DRONE_NAMES, 
            state='readonly', # Impede que o usuário digite no campo
            font=("Helvetica", 11)
        )
        self.drone_selector.grid(row=0, column=1, padx=5, pady=10, sticky='ew')

        # 2. Frame para os Botões
        button_frame = ttk.Frame(main_frame)
        button_frame.grid(row=1, column=0, columnspan=2, pady=15)

        # Botão para ver a deltas
        self.position_button = ttk.Button(button_frame, text="Ver deltas do drone", command=self.show_deltas)
        self.position_button.pack(side='left', padx=10)

        # Botão para ver a localização
        self.location_button = ttk.Button(button_frame, text="Ver Localização", command=self.show_location_info)
        self.location_button.pack(side='left', padx=10)

        # Botão para ver o status
        self.status_button = ttk.Button(button_frame, text="Ver Status", command=self.show_status_info)
        self.status_button.pack(side='left', padx=10)

        # 3. Label para exibir as informações
        self.info_label = ttk.Label(main_frame, text="Selecione um drone e clique em um botão.", font=("Helvetica", 12), foreground="gray")
        self.info_label.grid(row=2, column=0, columnspan=2, pady=20)
        
        # Configura o grid para expandir corretamente com a janela
        main_frame.columnconfigure(1, weight=1)

    # --- Funções que os botões irão chamar ---
    def get_selected_drone(self):
        """Retorna o nome do drone selecionado no menu."""
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
        """Exibe informação mockada sobre a localização."""
        drone_id = self.get_selected_drone()
        if not drone_id:
            self.info_label.config(text="Erro: Nenhum drone selecionado.", foreground="red")
            return

        # Valor mockado
        lat = f"-21"
        lon = f"-48"
        self.info_label.config(text=f"Localização do {drone_id}: Lat {lat}, Lon {lon}", foreground="blue")

    def show_status_info(self):
        """Exibe informação mockada sobre o status."""
        drone_id = self.get_selected_drone()
        if not drone_id:
            self.info_label.config(text="Erro: Nenhum drone selecionado.", foreground="red")
            return
            
        # Valor mockado
        statuses = ["Em voo", "Pousado", "Retornando à base", "Em manutenção"]
        self.info_label.config(text=f"Status do {drone_id}: {statuses[0]}", foreground="blue")

def setup_topology():
    """Creates and configures the network topology for the drone simulation."""
    info("--- Creating a Go drone network with Mininet-WiFi ---\n")
    net = Mininet_wifi(
        controller=Controller,
        link=wmediumd,
        wmediumd_mode=interference
    )
    net.addController('c0')
    info("*** Creating drone nodes ***\n")
    drones = []
    kwargs = {}
    kwargs['range'] = DRONE_RANGE
    for i, name in enumerate(DRONE_NAMES, 1):
        mac = f'00:00:00:00:00:0{i}'
        ip = f'10.0.0.{i}/8'
        DRONE_IPs.append(ip)
        drone = net.addStation(
            name,
            mac=mac,
            ip=ip,
            txpower=20,
            min_x=0, max_x=100,
            min_y=0, max_y=100,
            min_v=0.6*SPEED, max_v=SPEED, **kwargs
        )
        drones.append(drone)
    info("*** Configuring the signal propagation model ***\n")
    net.setPropagationModel(model="logDistance", exp=4)
    info("*** Configuring network nodes ***\n")
    net.configureNodes()
    net.plotGraph()
    net.setMobilityModel(time=0, model='RandomDirection',
                         max_x=100, max_y=100, seed=20)
    info("*** Adding ad-hoc links to drones ***\n")
    kwargs['proto'] = 'batman_adv'
    for drone in drones:
        net.addLink(
            drone,
            cls=adhoc,
            intf=f'{drone.name}-wlan0',
            ssid='adhocNet',
            mode='g',
            channel=5,
            ht_cap='HT40+', **kwargs
        )

    return net, drones

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
            command = f'curl -s --max-time 5 http://{drone.IP()}:{TCP_PORT}/state'
            response_str = drone.cmd(command).strip()
            position = drone.position
            writer = csv_writers[drone.name]

            ## Parse the JSON response and log the specific fields.
            try:
                data = json.loads(response_str)
                
                # Extract data, assuming the first entry in latest_readings is the relevant one
                reading_data = list(data['latest_readings'].values())[0]
                timestamp_ms = reading_data['timestamp']
                confidence = reading_data['confidence']
                all_deltas = data['all_deltas']

                for delta in all_deltas:
                    drone_delta_sets[i].add(json.dumps(delta))

                # Format the timestamp from milliseconds to a readable string
                formatted_timestamp = datetime.fromtimestamp(timestamp_ms / 1000).isoformat()
                
                # Convert all_deltas list to a compact JSON string for storage in a single CSV cell
                deltas_str = json.dumps(all_deltas)

                # Write the parsed data to the CSV file
                writer.writerow([formatted_timestamp, deltas_str, confidence, position, repetitions, convergence])
                
            except (json.JSONDecodeError, KeyError, IndexError) as e:
                # Handle cases where the response is not valid JSON or missing keys
                error_timestamp = datetime.now().isoformat()
                error_msg = f"ERROR parsing response: {type(e).__name__}"
                writer.writerow([error_timestamp, error_msg, 'N/A', position])
                info(f"-> ERROR for {drone.name}: Could not parse JSON response. See CSV for details.\n")
                info(f"   Problematic response: {response_str}\n")
        
        # Check for convergence
        repetitions += 1
        convergence = convergence_index(drone_delta_sets)
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

def main():
    """Main execution function."""
    try:
        os.system(f'sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null')
    except:
        pass
    
    setLogLevel('info')
    
    net, drones = setup_topology()
    
    info("*** Building the network ***\n")
    net.build()
    net.start()

    info("--- Starting Go applications on drones... ---\n")
    for i, drone in enumerate(net.stations, 1):
        drone_id = f'drone-go-{i}'
        command = (f"{EXEC_PATH} -id={drone_id} "
                   f"-tcp-port={TCP_PORT} -udp-port={UDP_PORT}")
        drone.cmd(f'xterm -e "{command}" &')

    info("\n*** Simulation is running. Type 'exit' or Ctrl+D to quit. ***\n")
    csv_files = {}
    csv_writers = {}
    
    # Use a try...finally block to ensure files are always closed properly.
    try:
        for drone in drones:
            filename = os.path.join(OUTPUT_DIR, f"{drone.name}_data.csv")
            # Open file in write mode with newline='' to prevent blank rows
            file_handle = open(filename, 'w', newline='')
            writer = csv.writer(file_handle)
            # Write the header row
            writer.writerow(['timestamp', 'all_deltas', 'confidence', 'position', 'repetition', 'convergence'])
            
            csv_files[drone.name] = file_handle
            csv_writers[drone.name] = writer
            info(f"Opened {filename} for data logging.\n")

        stop_event = threading.Event()
        fetch_thread = threading.Thread(target=fetch_states, args=(drones, stop_event, csv_writers), daemon=True)
        fetch_thread.start()

        root = tk.Tk()
        app = DroneControlPanel(root, drone_list=drones)
        root.mainloop()

        info("\n*** Simulation is running. CSV data is being saved in 'drone_execution_data'. ***\n")
        info("*** Type 'exit' or Ctrl+D in the CLI to quit. ***\n")
        CLI(net)

    finally:
        info("*** Shutting down simulation ***\n")
        if 'stop_event' in locals():
            stop_event.set()
        if 'fetch_thread' in locals():
            fetch_thread.join(timeout=5)
        
        # Close all open CSV files
        for file_handle in csv_files.values():
            file_handle.close()
        info("Closed all data log files.\n")

    info("*** Shutting down simulation ***\n")
    net.stop()

if __name__ == '__main__':
    main()
    # Final cleanup of any lingering Go processes
    os.system(f'sudo killall -9 {os.path.basename(EXEC_PATH)} &> /dev/null')
    info("--- Simulation finished ---\n")
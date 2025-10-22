from config import (
    X_MAX,
    Y_MAX,
)

def run_plot_graph(net):
    net.plotGraph(max_x=X_MAX, max_y=Y_MAX)

def run_energy_monitor(net, drones):
    net.plotEnergyMonitor(nodes=drones, single=True, title="FANET Energy Consumption")

def run_tx_bytes_telemetry(net, drones):
    net.telemetry(nodes=drones, data_type='tx_bytes', title="FANET TX BYTES")

def run_tx_dropped_telemetry(net, drones):
    net.telemetry(nodes=drones, data_type='tx_dropped', title="FANET TX DROPPED")
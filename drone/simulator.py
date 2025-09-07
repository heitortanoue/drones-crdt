#!/usr/bin/python

import time
import os
import shutil
from subprocess import Popen

from mininet.node import Controller
from mininet.log import setLogLevel, info
from mn_wifi.link import wmediumd, adhoc
from mn_wifi.net import Mininet_wifi
from mn_wifi.cli import CLI
from mn_wifi.telemetry import telemetry
from mn_wifi.wmediumdConnector import interference

# Lista global para rastrear os processos dos drones Go
go_drone_processes: list[Popen] = []
drone_names = []

# Diretório para salvar os dados de telemetria
output_dir = 'drone_execution_data/'

def update_telemetry_data_dir(names):
    """Move os arquivos de telemetria gerados para um diretório de saída."""
    info(f"*** Movendo arquivos de telemetria para {output_dir}... ***\n")
    os.makedirs(output_dir, exist_ok=True)
    for name in names:
        source_file = f"position-{name}-mn-telemetry.txt"
        destination_file = os.path.join(output_dir, source_file)
        
        if os.path.exists(source_file):
            shutil.move(source_file, destination_file)
            info(f"-> Arquivo {source_file} movido. <-\n")

def kill_go_processes():
    """Encerra todos os processos Go dos drones que foram iniciados."""
    info("*** Encerrando processos Go... ***\n")
    global go_drone_processes
    for process in go_drone_processes:
        if process.poll() is None:  # Verifica se o processo ainda está em execução
            info(f"-> Parando drone Go com PID {process.pid}... <-\n")
            process.terminate()
            process.wait()
    go_drone_processes = []
    info("*** Todos os drones Go foram parados. ***\n")

def topology():
    """Cria e executa a topologia de rede para a simulação dos drones."""
    info("--- Criando uma rede de drones Go com Mininet-WiFi ---\n")
    
    # Inicializa a rede com um controlador, link wmediumd para simulação wireless realista
    net = Mininet_wifi(controller=Controller, link=wmediumd, wmediumd_mode=interference)

    info("*** Criando nós para representar cada drone ***\n")
    # Posições iniciais dos drones
    dr1 = net.addStation('dr1', mac='00:00:00:00:00:01', ip='10.0.0.1/8', position='30,60,0')
    dr2 = net.addStation('dr2', mac='00:00:00:00:00:02', ip='10.0.0.2/8', position='70,30,0')
    dr3 = net.addStation('dr3', mac='00:00:00:00:00:03', ip='10.0.0.3/8', position='10,20,0')

    # Adiciona um controlador para gerenciar a rede
    c0 = net.addController('c0')

    info("*** Configurando o modelo de propagação do sinal ***\n")
    net.setPropagationModel(model="logDistance", exp=4.5)

    info("*** Configurando os nós da rede ***\n")
    net.configureNodes()

    info("*** Adicionando links ad-hoc para comunicação drone-a-drone ***\n")
    # A rede ad-hoc com batman_adv cria uma malha (mesh) onde os drones
    # podem se comunicar diretamente, ideal para o protocolo de gossip.
    net.addLink(dr1, cls=adhoc, intf='dr1-wlan0', ssid='adhocNet', proto='batman_adv', mode='g', channel=5, ht_cap='HT40+')
    net.addLink(dr2, cls=adhoc, intf='dr2-wlan0', ssid='adhocNet', proto='batman_adv', mode='g', channel=5, ht_cap='HT40+')
    net.addLink(dr3, cls=adhoc, intf='dr3-wlan0', ssid='adhocNet', proto='batman_adv', mode='g', channel=5, ht_cap='HT40+')

    info("*** Construindo a rede ***\n")
    net.build()
    c0.start() # Inicia o controlador

    # Habilita a telemetria para visualização da posição dos drones em tempo real
    nodes = net.stations
    telemetry(nodes=nodes, single=True, data_type='position')

    for n in net.stations:
        drone_names.append(n.name)

    info("--- Iniciando as aplicações Go nos drones... ---\n")
    global go_drone_processes
    # O executável 'main' (compilado do seu código Go) deve estar no mesmo diretório
    for i, drone in enumerate(net.stations, 1):
        drone_id = f'drone-go-{i}'
        udp_port = 7000 + i
        tcp_port = 8080 + i
        # Inicia o processo Go em cada nó da simulação
        process = drone.popen(f'./main -id={drone_id} -udp-port={udp_port} -tcp-port={tcp_port}')
        go_drone_processes.append(process)
        info(f"*** Drone {drone_id} iniciado com PID: {process.pid} (UDP:{udp_port}, TCP:{tcp_port}) ***\n")

    time.sleep(5)  # Aguarda a inicialização completa das aplicações Go

    info("*** A simulação está em execução. Use a CLI para interagir com os drones. ***\n")
    info("-> Exemplo para mover o drone dr1: py dr1.position = (50, 50, 0)\n")
    info("-> Para visualizar os logs de um drone: dr1 xterm\n")
    CLI(net)

    info("*** Encerrando a simulação ***\n")
    kill_go_processes()
    net.stop()
    update_telemetry_data_dir(drone_names)

if __name__ == '__main__':
    setLogLevel('info')
    # Garante que processos de simulações anteriores sejam encerrados
    os.system('sudo killall -9 main') 
    topology()
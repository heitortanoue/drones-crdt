#!/usr/bin/python

import time
import os
import subprocess
import signal

from mininet.log import setLogLevel, info
from mn_wifi.link import wmediumd, adhoc
from mn_wifi.cli import CLI
from mn_wifi.net import Mininet_wifi
from mn_wifi.telemetry import telemetry
from mn_wifi.wmediumdConnector import interference


# Lista global para armazenar os processos dos drones Go
go_drone_processes = []

def topology():
    "Cria uma rede de m\u00FAltiplos drones Go."
    net = Mininet_wifi(link=wmediumd, wmediumd_mode=interference)

    info("*** Criando n\u00F3s (drones)\n")
    dr1 = net.addStation('dr1', mac='00:00:00:00:00:01', ip='10.0.0.1/8',
                         position='30,60,0')
    dr2 = net.addStation('dr2', mac='00:00:00:00:00:02', ip='10.0.0.2/8',
                         position='70,30,0')
    dr3 = net.addStation('dr3', mac='00:00:00:00:00:03', ip='10.0.0.3/8',
                         position='10,20,0')

    net.setPropagationModel(model="logDistance", exp=4.5)

    info("*** Configurando n\u00F3s\n")
    net.configureNodes()

    # Adicionando links ad-hoc para todos os drones
    net.addLink(dr1, cls=adhoc, intf='dr1-wlan0',
                ssid='adhocNet', proto='batman_adv',
                mode='g', channel=5, ht_cap='HT40+')

    net.addLink(dr2, cls=adhoc, intf='dr2-wlan0',
                ssid='adhocNet', proto='batman_adv',
                mode='g', channel=5, ht_cap='HT40+')

    net.addLink(dr3, cls=adhoc, intf='dr3-wlan0',
                ssid='adhocNet', proto='batman_adv',
                mode='g', channel=5, ht_cap='HT40+')


    info("*** Iniciando rede\n")
    net.build()

    # Configura o modelo de mobilidade para os drones se moverem
    info("*** Habilitando mobilidade para os drones\n")
    net.setMobilityModel(time=0, model='RandomDirection', max_x=100, max_y=100, min_v=0.1, max_v=0.5)

    nodes = net.stations
    telemetry(nodes=nodes, single=True, data_type='position')

    sta_drone = []
    for n in net.stations:
        sta_drone.append(n.name)
    sta_drone_send = ' '.join(map(str, sta_drone))

    info("*** Iniciando socket server\n")
    net.socketServer(ip='127.0.0.1', port=12345)

    # Inicia m\u00FAltiplos drones Go, cada um com ID e portas \u00FAnicos
    info("*** Iniciando os drones Go...\n")
    global go_drone_processes
    # O c\u00F3digo Go deve ser compilado primeiro e o execut\u00E1vel deve estar no path
    for i, drone in enumerate(net.stations, 1):
        drone_id = f'drone-go-{i}'
        udp_port = 7000 + i
        tcp_port = 8080 + i
        # Inicia o processo do drone Go no n\u00F3 Mininet e armazena o processo
        process = drone.popen(f'./main -id={drone_id} -udp-port={udp_port} -tcp-port={tcp_port}')
        go_drone_processes.append(process)
        info(f"*** Drone {drone_id} iniciado com PID: {process.pid} (UDP:{udp_port}, TCP:{tcp_port})\n")

    time.sleep(5)  # Espera os drones Go inicializarem

    info("*** Executando CLI\n")
    CLI(net)

    info("*** Parando rede\n")
    kill_process()
    net.stop()


def kill_process():
    path = os.path.dirname(os.path.abspath(__file__))
    info("*** Parando processos...\n")

    # Encerra os processos dos drones Go
    global go_drone_processes
    for process in go_drone_processes:
        if process.poll() is None:  # Verifica se o processo ainda est\u00E1 rodando
            info(f"*** Parando o drone Go com PID {process.pid}...\n")
            process.terminate()
            process.wait()
    go_drone_processes = []
    info("*** Todos os drones Go foram parados.\n")

    os.system('rm -rf {}/data/*'.format(path))


if __name__ == '__main__':
    setLogLevel('info')
    # Matando processos antigos
    kill_process()
    topology()

#!/usr/bin/python

import time
import os
import curses
import shutil

from mininet.log import setLogLevel, info
from mn_wifi.link import wmediumd, adhoc
from mn_wifi.net import Mininet_wifi
from mn_wifi.telemetry import telemetry
from mn_wifi.wmediumdConnector import interference

# Lista global para armazenar os processos dos drones Go
go_drone_processes = []

# Diretório para salvar os dados de telemetria
output_dir = 'drone_execution_data/'

def keyboard_control(net):
    """
    Permite o controle manual dos drones usando o teclado com a biblioteca curses
    """
    drones = net.stations
    selected_drone_index = 0
    move_step = 2  # Passo de movimento em unidades de posição

    stdscr = curses.initscr()
    curses.cbreak()
    stdscr.keypad(True)
    stdscr.nodelay(True)
    curses.noecho()

    info("*** Iniciando controle manual dos drones ***\n")
    info("Use as SETAS para mover, TAB para trocar de drone, 'q' para sair.\n")

    try:
        while True:
            drone = drones[selected_drone_index]
            
            stdscr.clear()
            stdscr.addstr(0, 0, "Controle de Drones Ativado")
            stdscr.addstr(1, 0, "Pressione 'q' para sair e parar a simulação.")
            stdscr.addstr(3, 0, f"--> Drone selecionado: {drone.name} [IP: {drone.IP()}]")
            
            for i, d in enumerate(drones):
                current_pos_str = d.position if hasattr(d, 'position') else "N/A"
                if i != selected_drone_index:
                    stdscr.addstr(4 + i, 0, f"    Drone: {d.name} [Posição: {current_pos_str}]")

            current_pos = drone.position
            stdscr.addstr(4 + selected_drone_index, 0, f"--> Drone: {drone.name} [Posição: {current_pos}] <--")
            stdscr.refresh()

            key = stdscr.getch()

            if key == ord('q'):
                break

            elif key == ord('\t'):
                selected_drone_index = (selected_drone_index + 1) % len(drones)
                time.sleep(0.1)
            
            elif key in [curses.KEY_UP, curses.KEY_DOWN, curses.KEY_LEFT, curses.KEY_RIGHT]:
                x, y, z = float(current_pos[0]), float(current_pos[1]), float(current_pos[2])
                
                if key == curses.KEY_UP:
                    y += move_step
                elif key == curses.KEY_DOWN:
                    y -= move_step
                elif key == curses.KEY_LEFT:
                    x -= move_step
                elif key == curses.KEY_RIGHT:
                    x += move_step
                
                drone.position = (x, y, z)
            
            time.sleep(0.1)
    finally:
        curses.nocbreak()
        stdscr.keypad(False)
        curses.echo()
        curses.endwin()
        info("\n*** Controle manual finalizado. ***\n")

def update_telemetry_data_dir(nodes):
    info(f"*** Movendo arquivos de telemetria para {output_dir}... ***\n")
    os.makedirs(output_dir, exist_ok=True)
    for drone_node in nodes:
        source_file = f"position-{drone_node.name}-mn-telemetry.txt" 
        destination_file = os.path.join(output_dir, source_file)
        
        if os.path.exists(source_file):
            shutil.move(source_file, destination_file)
            info(f"-> Arquivo {source_file} movido. <-\n")

def topology():
    info("--- Criando uma rede de multiplos drones Go com Mininet-WiFi ---\n")
    net = Mininet_wifi(link=wmediumd, wmediumd_mode=interference)

    info("*** Criando nós que representam cada um dos drones ***\n")
    dr1 = net.addStation('dr1', mac='00:00:00:00:00:01', ip='10.0.0.1/8',
                         position='30,60,0')
    dr2 = net.addStation('dr2', mac='00:00:00:00:00:02', ip='10.0.0.2/8',
                         position='70,30,0')
    dr3 = net.addStation('dr3', mac='00:00:00:00:00:03', ip='10.0.0.3/8',
                         position='10,20,0')

    net.setPropagationModel(model="logDistance", exp=4.5)

    info("*** Configurando nós ***\n")
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


    info("*** Iniciando rede ***\n\n")
    net.build()

    nodes = net.stations
    telemetry(nodes=nodes, single=True, data_type='position')

    sta_drone = []
    for n in net.stations:
        sta_drone.append(n.name)
    sta_drone_send = ' '.join(map(str, sta_drone))

    info("*** Iniciando socket server ***\n")
    net.socketServer(ip='127.0.0.1', port=12345)

    info("--- Iniciando os drones Go... ---\n")
    global go_drone_processes
    # O código Go deve ser compilado primeiro e o executável deve estar no path
    for i, drone in enumerate(net.stations, 1):
        drone_id = f'drone-go-{i}'
        udp_port = 7000 + i
        tcp_port = 8080 + i
        process = drone.popen(f'./main -id={drone_id} -udp-port={udp_port} -tcp-port={tcp_port}')
        go_drone_processes.append(process)
        info(f"*** Drone {drone_id} iniciado com PID: {process.pid} (UDP:{udp_port}, TCP:{tcp_port}) ***\n")

    time.sleep(5)  # Espera os drones Go inicializarem

    info("*** Iniciando controle manual. Abra a visualização de telemetria em outro terminal ***\n")
    keyboard_control(net)

    info("*** Parando rede ***\n")
    kill_process()
    update_telemetry_data_dir(net.stations)
    net.stop()


def kill_process():
    path = os.path.dirname(os.path.abspath(__file__))
    info("Parando processos...\n")

    global go_drone_processes
    for process in go_drone_processes:
        if process.poll() is None:
            info(f"*** Parando o drone Go com PID {process.pid}... ***\n")
            process.terminate()
            process.wait()
    go_drone_processes = []
    info("*** Todos os drones Go foram parados. ***\n")

    os.system('rm -rf {}/data/*'.format(path))


if __name__ == '__main__':
    setLogLevel('info')
    kill_process()
    topology()
#!/usr/bin/env python3
# author: https://github.com/macdylan
import os
import sys
import time
import socket
import tempfile
import threading

try:
    import requests
except:
    print("Please execute: pip3 install requests")
    sys.exit(1)


def discover(msg=b"discover", port=20054, timeout=4):
    cs = socket.socket(socket.AF_INET, socket.SOCK_DGRAM, socket.IPPROTO_UDP)
    cs.settimeout(timeout)
    cs.setsockopt(socket.SOL_SOCKET, socket.SO_BROADCAST, 1)
    cs.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    cs.setsockopt(socket.IPPROTO_IP, socket.IP_MULTICAST_TTL, 255)
    cs.sendto(msg, ("<broadcast>", port))

    servers = []

    try:
        while True:
            resp, addr = cs.recvfrom(512)
#            print(resp.decode("utf-8"))
            ip, _ = addr
            servers.append((ip, resp))
        return servers
    except socket.timeout as e:
        return servers


def select_server():
    print("Discovering ...\n")

    fcache = os.path.join(tempfile.gettempdir(), "sm2.ip")
    try:
        fd = open(fcache, "r")
        ip = fd.readline().strip()
        if ip:
            return ip
    except FileNotFoundError:
        pass
    finally:
        unlink(fcache)

    server = discover()

    if len(server) == 1:
        return server[0][0] # server ip

    elif len(server) > 1:
        print("Found %d machines:" % len(server))
        for ip, resp in server:
            print("> %s [ip: %s]" % (resp.decode(), ip))
        print("\nUse 'sm2uploader.py /path/to/file ip' to specify the target machine")

    elif len(server) == 0:
        print("No machines detected.")
        print("Please check the touchscreen and tap Disconnect.")

    return None


def upload(fpath, server):
    if os.path.isdir(fpath):
        print("'%s' is a directory." % fpath)
        return None

    token = ""
    fsize = os.path.getsize(fpath)
    ftoken = os.path.join(tempfile.gettempdir(), "sm2.token")

    if os.path.isfile(ftoken):
        token = open(ftoken).readline().strip()

    endpoint = "http://%s:8080/api/v1" % server

    token = connect(endpoint, token)
    if not token:
        unlink(ftoken)
        print("Please check the touchscreen and tap Disconnect.")
        return None

    open(ftoken, mode="w").write(token)
    if not check_status(endpoint, token):
        # expired
        unlink(ftoken)
        print("⚠️ Screen authorization needed.")
        return None

    print("IP Address\t: %s" % server)
    print("Token\t\t: %s" % token)
    print("Payload\t\t: %s" % fpath)
    print("Payload size(b)\t: %d" % fsize)

    _start_heartbeat(endpoint, token)

    print("\nSending ... ", end="")
    conn = requests.post(url=endpoint+"/upload",
        data={"token": token},
        files={"file": open(fpath, "rb")},
        timeout=40)
    if conn.status_code == 200:
        _stop_heartbeat()
        disconnect(endpoint, token)
        open(os.path.join(tempfile.gettempdir(), "sm2.ip"), "w").write(server)

        print("Success ✅")
        print("Start print this file on the touchscreen.")
        return True

    else:
        print("Failed ❌")
        print("%d: %s" % (conn.status_code, conn.text))
        return False

def connect(endpoint, token):
    try:
        conn = requests.post(url=endpoint+"/connect", data={"token": token})
        if conn.status_code == 200:
            return conn.json().get("token")

    except requests.exceptions.ConnectionError as e:
        print("Error: %s" % e)


def disconnect(endpoint, token):
    try:
        requests.post(url=endpoint+"/disconnect", data={"token": token})
    except requests.exceptions.ConnectionError as e:
        pass
        # print("Error: %s" % e)


def check_status(endpoint, token):
    try:
        tip = True
        while True:
            conn = requests.get(url=endpoint+"/status", params={"token": token})
            if conn.status_code == 200:
                # print("[200]%s" % conn.text)
                return True

            if conn.status_code == 204 and tip:
                print("Please tap Yes on Snapmaker touchscreen to continue.")
                tip = False

            if conn.status_code == 401:
                return False

            time.sleep(1)
    except:
        return False


def unlink(fpath):
    try:
        os.unlink(fpath)
    except FileNotFoundError:
        pass


__heartbeat_thread = None
__heartbeat_stop = False
def _start_heartbeat(endpoint, token):
    global __heartbeat_thread
    global __heartbeat_stop
    def _check():
        while not __heartbeat_stop:
            # print("heartbeat")
            requests.get(url=endpoint+"/status", params={"token": token})
            time.sleep(3)

    if not __heartbeat_thread or not __heartbeat_thread.is_alive():
        __heartbeat_stop = False
        __heartbeat_thread = threading.Thread(target=_check, daemon=True)
        __heartbeat_thread.start()


def _stop_heartbeat():
    global __heartbeat_thread
    global __heartbeat_stop
    __heartbeat_stop = True
    if __heartbeat_thread:
        __heartbeat_thread.join()


if __name__ == "__main__":
    res = None
    if len(sys.argv) == 3:
        res = upload(sys.argv[1], sys.argv[2])

    elif len(sys.argv) == 2:
        server = select_server()
        if not server:
            sys.exit(255)
        res = upload(sys.argv[1], server)

    else:
        print(select_server())
        #print("Usage: sm2uploader.py /path/to/file [ip]")
        sys.exit(1)

    if not res:
        sys.exit(255)

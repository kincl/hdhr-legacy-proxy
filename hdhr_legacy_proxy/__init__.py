from gevent import monkey, socket
monkey.patch_all() # we need to patch very early

import os
import sys
import time
import pickle
import logging

from flask import Flask, jsonify, Response
from hdhr.adapter import HdhrUtility, HdhrDeviceQuery

from hdhr_legacy_proxy.util import format_msg

app = Flask(__name__)
application = app

handler = app.logger.handlers[0]
formatter = logging.Formatter('[%(asctime)s] [%(process)d] [%(levelname)s] [%(name)s] %(message)s', "%Y-%m-%d %H:%M:%S %z")
handler.setFormatter(formatter)
app.logger.addHandler(handler)
app.logger.setLevel(logging.INFO)

devices = HdhrUtility.discover_find_devices_custom()
if len(devices) == 1:
    device = devices[0]
    app.logger.info(f"found device: [{device.nice_device_id}] {device.nice_ip}")
else:
    app.logger.error("error finding a device")
    sys.exit(1)

config = {
    "proxy_host": os.environ.get("HDHR_LEGACY_PROXY_HOST") or "",
    "proxy_port": os.environ.get("HDHR_LEGACY_PROXY_PORT") or "8000",
    "proxy_tuner_port": os.environ.get("HDHR_LEGACY_PROXY_TUNER_PORT") or "5000",
    "tuners": 1
}

def do_scan(as_bytes=True):
    dev = HdhrDeviceQuery(HdhrUtility.device_create_from_str(device.nice_device_id))

    yield format_msg("Starting channel scan", as_bytes)
    channels = []
    scanner = dev.scan_channels(bytes('us-bcast', 'utf-8'))
    for channel in scanner:
        if channel[0]:
            programs = [channel[3].programs[i].program_str.decode('ascii') for i in range(channel[3].program_count)]
            yield format_msg(f"Found channel {channel[1] + 1}/{channel[2] + 1}: {', '.join(programs)}", as_bytes)
        else:
            yield format_msg(f"Scanned channel {channel[1] + 1}/{channel[2] + 1}", as_bytes)
        if channel[0] == True:
            channels.append(channel[3])
    yield format_msg("Completed channel scan", as_bytes)

    app.logger.info("Writing new channels.pkl")
    with open('channels.pkl', 'wb') as f:
        pickle.dump(channels, f)

try:
    with open('channels.pkl', 'r+') as f:
        pickle.load(f)
except OSError:
    app.logger.info("No previous channels.pkl found!")
    for channel in do_scan(as_bytes=False):
        app.logger.info(channel)

sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.bind(("0.0.0.0", int(config['proxy_tuner_port'])))
app.logger.info(f"listening on UDP port :{config['proxy_tuner_port']}")

proxy_url = f"http://{config['proxy_host']}:{config['proxy_port']}"

discoverData = {
    "FriendlyName": "hdhrLegacyProxy",
    "ModelNumber": "HDTC-2US",
    "FirmwareName": "hdhomeruntc_atsc",
    "TunerCount": config["tuners"],
    "FirmwareVersion": "20150826",
    "DeviceID": "12345678",
    "DeviceAuth": "test1234",
    "BaseURL": proxy_url,
    "LineupURL": f"{proxy_url}/lineup.json"
}

@app.route("/discover.json")
def discover():
    return jsonify(discoverData)


@app.route("/lineup_status.json")
def status():
    return jsonify({
        "ScanInProgress": 0,
        "ScanPossible": 1,
        "Source": "Antenna",
        "SourceList": ["Antenna"]
    })


@app.route('/lineup.json')
def lineup():
    lineup = []

    try:
        with open('channels.pkl', 'rb+') as f:
            channels = pickle.load(f)
    except OSError:
        app.logger.error("No previous channels.pkl found!")
        return jsonify(lineup)

    for channel in channels:
        for program in channel.programs:
            if len(program.program_str) == 0:
                continue
            lineup.append({
                "GuideNumber": program.program_str.decode('ascii').split(' ')[1],
                "GuideName": program.name.decode('ascii'),
                "URL": f"{proxy_url}/auto/{channel.frequency}/{program.program_number}"
            })

    return jsonify(lineup)


@app.route('/auto/<channel>/<program>')
def stream(channel, program):
    app.logger.info(f"GET /auto/{channel}/{program} starting stream")
    # dir = os.getcwd()
    # os.system(f"/bin/bash -c {dir}/test_tv.sh")

    dev = HdhrDeviceQuery(HdhrUtility.device_create_from_str(device.nice_device_id))
    dev.set_tuner_channel(channel)
    dev.set_tuner_program(program)
    dev.set_tuner_target(f"udp://{config['proxy_host']}:{config['proxy_tuner_port']}")

    app.logger.info("waiting 2 seconds")
    time.sleep(2)

    _, status = dev.get_tuner_status()
    app.logger.info(f"tuner status: {status.decode('ascii')}")
    # app.logger.info(f"stream info: {dev.get_tuner_streaminfo()}")
    
    # TODO check that tuner is set

    def generate():
        yield bytes()
        # print(f"data: {data}")
        while True:
            data, addr = sock.recvfrom(1316)
            # print(f"received from {addr[0]}:{addr[1]}")
            yield data
    return Response(generate(), content_type="video/mpeg", direct_passthrough=True)


@app.route('/scan')
def scan_channels():
    app.logger.info("Scanning channels")

    try:
        with open('channels.pkl', 'r+') as f:
            channels = pickle.load(f)
            return len(channels)
    except OSError:
        app.logger.info("No previous channels.pkl found!")
        return Response(do_scan(), content_type="text/plain", direct_passthrough=True)

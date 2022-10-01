from gevent import monkey, socket
monkey.patch_all() # we need to patch very early

import pickle
from flask import Flask, jsonify, Response

from hdhr.adapter import HdhrUtility, HdhrDeviceQuery

sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
sock.bind(("0.0.0.0", 5000))
app = Flask(__name__)

config = {
    "host": "http://192.168.5.111:8000",
    "tuners": 1
}

discoverData = {
    "FriendlyName": "hdhrLegacyProxy",
    "ModelNumber": "HDTC-2US",
    "FirmwareName": "hdhomeruntc_atsc",
    "TunerCount": config["tuners"],
    "FirmwareVersion": "20150826",
    "DeviceID": "12345678",
    "DeviceAuth": "test1234",
    "BaseURL": f"{config['host']}",
    "LineupURL": f"{config['host']}/lineup.json"
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
        with open('channels.dat', 'rb+') as f:
            channels = pickle.load(f)
    except OSError:
        print("No previous channels.dat found!")
        return jsonify(lineup)

    for channel in channels:
        for program in channel.programs:
            if len(program.program_str) == 0:
                continue
            lineup.append({
                "GuideNumber": program.program_str.decode('ascii').split(' ')[1],
                "GuideName": program.name.decode('ascii'),
                "URL": f"{config['host']}/auto/{channel.frequency}/{program.program_number}"
            })

    return jsonify(lineup)


@app.route('/auto/<channel>/<program>')
def stream(channel, program):
    # dir = os.getcwd()
    # os.system(f"/bin/bash -c {dir}/test_tv.sh")

    devices = HdhrUtility.discover_find_devices_custom()

    dev = HdhrDeviceQuery(HdhrUtility.device_create_from_str(devices[0].nice_device_id))
    dev.set_tuner_channel(channel)
    dev.set_tuner_program(program)
    dev.set_tuner_target("udp://192.168.5.111:5000")
    
    # print(dev.get_tuner_status())
    # print(dev.get_tuner_streaminfo())

    def generate():
        yield bytes()
        # print(f"data: {data}")
        while True:
            data, addr = sock.recvfrom(1500)
            # print(f"received from {addr[0]}:{addr[1]}")
            yield data
    return Response(generate(), content_type="video/mpeg", direct_passthrough=True)


@app.route('/scan')
def scan_channels():
    devices = HdhrUtility.discover_find_devices_custom()
    dev = HdhrDeviceQuery(HdhrUtility.device_create_from_str(devices[0].nice_device_id))

    try:
        with open('channels.dat', 'rb+') as f:
            channels = pickle.load(f)
    except OSError:
        print("No previous channels.dat found!")
        channels = dev.scan_channels(bytes('us-bcast', 'utf-8'))
        print("Writing new channels.dat")
        with open('channels.dat', 'wb') as f:
            pickle.dump(channels, f)

    return len(channels)

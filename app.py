from gevent import monkey, socket
monkey.patch_all() # we need to patch very early

import os
from flask import Flask, jsonify, Response


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

    # for c in _get_channels():
    #       c = c['channel']
    #       url = '%s/auto/v%s' % (config['npvrProxyURL'], c['channelNum'])

    #       lineup.append({'GuideNumber': str(c['channelNum']),
    #                      'GuideName': c['channelName'],
    #                      'URL': url
    #                      })

    lineup.append({
        "GuideNumber": "1",
        "GuideName": "TEST",
        "URL": f"{config['host']}/auto/v1"
    })

    return jsonify(lineup)

@app.route('/auto/<channel>')
def stream(channel):
    dir = os.getcwd()
    os.system(f"/bin/bash -c {dir}/test_tv.sh")
    def generate():
        yield bytes()
        # print(f"data: {data}")
        while True:
            data, addr = sock.recvfrom(1500)
            # print(f"received from {addr[0]}:{addr[1]}")
            yield data
    return Response(generate(), content_type="video/mpeg", direct_passthrough=True)

import sys
import pickle
import logging
default_logger = logging.getLogger()
default_logger.setLevel(logging.DEBUG)
handler = logging.StreamHandler(sys.stdout)
formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
handler.setFormatter(formatter)
default_logger.addHandler(handler)


# TODO get rid of this
from hdhr.adapter import HdhrUtility, HdhrDeviceQuery
def debug():
    devices = HdhrUtility.discover_find_devices_custom()

    dev = HdhrDeviceQuery(HdhrUtility.device_create_from_str(devices[0].nice_device_id))
    # print(dev.get_tuner_status())
    # print(dev.get_tuner_streaminfo())
    # # print(dev.get_tuner_vstatus())
    # print(dev.get_tuner_program())
    # print(dev.get_supported())
    
    # dev.set_tuner_channel("auto:34")
    # dev.set_tuner_program("1")
    
    # print(dev.get_tuner_status())
    # print(dev.get_tuner_streaminfo())

    # try:
    #     with open('channels.dat', 'rb+') as f:
    #         channels = pickle.load(f)
    # except OSError:
    #     print("No previous channels.dat found!")
    #     channels = dev.scan_channels(bytes('us-bcast', 'utf-8'))
    #     print("Writing new channels.dat")
    #     with open('channels.dat', 'wb') as f:
    #         pickle.dump(channels, f)

    channels = dev.scan_channels(bytes('us-bcast', 'utf-8'))

    # print(channels)
    for channel in channels:
        print(channel)
        # for program in channel.programs:
        #     if len(program.program_str) == 0:
        #         continue
        #     print(program.program_str.decode('ascii').split(' ')[1])

if __name__ == "__main__":
    debug()

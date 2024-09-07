#!/bin/sh
# this is run from /tmp by the HDHomerun installer
cd "${0%/[^/]*}"
reset

export PATH=.:/usr/local/bin:$PATH

probe_devices() {
	echo "Probing for devices:"
	DEVICES=($(hdhomerun_config discover -4 --dedupe | awk '/^hdhomerun device/ { print $6; }'))
	show_devices
}

show_devices() {
	for (( i=0; i<${#DEVICES[*]}; i++ )); do {
                DEVICEID[$i]=$(hdhomerun_config discover "${DEVICES[$i]}" | awk '/^hdhomerun device/ { print $3; }')
		VERSIONS[$i]=$(hdhomerun_config "${DEVICES[$i]}" get /sys/version)
		  MODELS[$i]=$(hdhomerun_config "${DEVICES[$i]}" get /sys/model || echo hdhomerun_atsc)
		echo "${i}: ${DEVICEID[$i]} firmware ${VERSIONS[$i]}"
	}; done

	echo "${#DEVICES[*]} device(s) found"
	echo ""

	[[ ${#DEVICES[*]} -eq 0 ]] && {
		echo "No HDHomeRun devices detected on your network."
		echo "Please check the network configuration and try again."
		echo ""
		exit 0
	}
}

upgrade() {
	for (( i=0; i<${#DEVICES[*]}; i++ )); do {
		local firmware=$(ls -1 "${MODELS[$i]}"_firmware_*.bin 2>/dev/null | sort -n | tail -n 1)
		local fwver=$(echo "${firmware}" | sed "s/${MODELS[$i]}_firmware_\(.*\)\.bin/\1/")

		[[ -z "${fwver}" ]] && {
			echo "${DEVICEID[$i]}(${MODELS[$i]}) no firmware upgrade found"
			continue
		}

		[[ ( "${fwver}" == "${VERSIONS[$i]}" ) || ( "${fwver}" < "${VERSIONS[$i]}" ) ]] && {
			echo "${DEVICEID[$i]}(${MODELS[$i]}) is already running the latest firmware"
			continue
		}

		echo "${DEVICEID[$i]}(${MODELS[$i]}) => $fwver"
		hdhomerun_config "${DEVICES[$i]}" upgrade "${firmware}"
		echo ""
	}; done

	sleep 5

	echo ""
	echo "Upgrade results:"
	show_devices
}

probe_devices
upgrade

echo "Finished."
echo "The HDHomeRun application can be found in your Applications folder"
echo "(You can close this window now)"
echo ""

exit 0

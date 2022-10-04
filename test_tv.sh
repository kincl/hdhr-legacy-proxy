#!/bin/bash 
# This is a shim right now to configure the HDHR to start sending video to the proxy

echo "trying Channel 34 Program 1"
CHANNEL=34
PROGRAM=1
UDP_PORT=6000
TUNER=0

# device id of homerun
DEVICE=`hdhomerun_config discover | awk '{print $3}'`

# this computer's ip address I want the Homerun to connect to
MY_IP=`ifconfig en0 | grep inet | grep -v inet6 | awk '{print $2}'`
if [ -z "$MY_IP" ]; then
    MY_IP=`ifconfig en1 | grep inet | grep -v inet6 | awk '{print $2}'`
fi

# set the tuner channel
hdhomerun_config $DEVICE set /tuner$TUNER/channel auto:$CHANNEL

# set the program id
hdhomerun_config $DEVICE set /tuner$TUNER/program $PROGRAM

# tell it to send the video stream our way
hdhomerun_config $DEVICE set /tuner$TUNER/target udp://$MY_IP:$UDP_PORT

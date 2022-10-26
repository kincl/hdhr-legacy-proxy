package device

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"
	"unsafe"
)

// #cgo CFLAGS: -I../../libhdhomerun
// #cgo LDFLAGS: -lpthread -L../../libhdhomerun -lhdhomerun
// #include <stdio.h>
// #include <stdbool.h>
// #include <stdint.h>
// #include "hdhomerun.h"
import "C"

type Device struct {
	hdhrDevice *C.struct_hdhomerun_device_t
}

type ProgramScan struct {
	Program_str    string `json:"program_str"`
	Name           string `json:"name"`
	Program_number int    `json:"program_number"`
}

type ChannelScan struct {
	Channel_str string        `json:"channel_str"`
	Frequency   int           `json:"frequency"`
	Programs    []ProgramScan `json:"programs"`
}

func (device *Device) FindDevices() {
	wildcard, _ := strconv.ParseInt("0xFFFFFFFF", 0, 64)
	ptr := C.malloc(C.sizeof_struct_hdhomerun_discover_device_t)
	defer C.free(unsafe.Pointer(ptr))

	var discovered *C.struct_hdhomerun_discover_device_t
	var numFound C.int

	for {
		discovered = (*C.struct_hdhomerun_discover_device_t)(ptr)
		numFound = C.hdhomerun_discover_find_devices_custom_v2(
			C.uint(0),
			C.uint(wildcard),
			C.uint(wildcard),
			discovered,
			1)

		if numFound != 0 {
			break
		}

		log.Println("Did not find any devices! Trying again in 1s...")
		time.Sleep(time.Second * 1)
	}

	//TODO numFound can return -1 if the library has errors creating a socket

	log.Printf("Found %d HDHR device: %X %s\n", numFound, discovered.device_id, inet_ntoa((uint32)(discovered.ip_addr)))
	device.hdhrDevice = C.hdhomerun_device_create(discovered.device_id, discovered.ip_addr, C.uint(0), nil)
}

func (device *Device) SetChannel(channel string, program string, target string) error {
	tunerChannel := C.CString(fmt.Sprintf("auto:%s", channel))
	channel_ok := C.hdhomerun_device_set_tuner_channel(device.hdhrDevice, tunerChannel)
	C.free(unsafe.Pointer(tunerChannel))
	if channel_ok == 0 {
		// log.Println("Unable to set tuner channel!")
		return errors.New("unable to set tuner channel")
	}

	tunerProgram := C.CString(program)
	program_ok := C.hdhomerun_device_set_tuner_program(device.hdhrDevice, tunerProgram)
	C.free(unsafe.Pointer(tunerProgram))
	if program_ok == 0 {
		// log.Println("Unable to set tuner program!")
		return errors.New("unable to set tuner program")
	}

	tunerTarget := C.CString(fmt.Sprintf("udp://%s", target))
	target_ok := C.hdhomerun_device_set_tuner_target(device.hdhrDevice, tunerTarget)
	C.free(unsafe.Pointer(tunerTarget))
	if target_ok == 0 {
		// log.Println("Unable to set tuner target!")
		return errors.New("unable to set tuner target")
	}

	return nil
}

func (device *Device) Scan() ([]ChannelScan, error) {
	channelList := C.hdhomerun_channel_list_create(C.CString("us-bcast"))
	totalNum := (int)(C.hdhomerun_channel_list_total_count(channelList))

	channelScan := C.channelscan_create(device.hdhrDevice, C.CString("us-bcast"))

	channels := []ChannelScan{}

	for i := 0; i <= totalNum; i++ {
		log.Printf("scanning %d/%d", i+1, totalNum+1)
		ptr := C.malloc(C.sizeof_struct_hdhomerun_channelscan_result_t)
		defer C.free(unsafe.Pointer(ptr))
		result := (*C.struct_hdhomerun_channelscan_result_t)(ptr)

		advanceOK := C.channelscan_advance(channelScan, result)
		if advanceOK != 1 {
			break
		}

		detectOK := C.channelscan_detect(channelScan, result)
		if detectOK == 1 && result.program_count > 0 {
			channel := C.GoString(&result.channel_str[0])
			var programs []ProgramScan
			log.Printf("Found %s\n", channel)

			for j := 0; j < int(result.program_count); j++ {
				programs = append(programs, ProgramScan{
					Program_str:    C.GoString(&result.programs[j].program_str[0]),
					Name:           C.GoString(&result.programs[j].name[0]),
					Program_number: int(result.programs[j].program_number),
				})
			}

			channels = append(channels, ChannelScan{
				Channel_str: channel,
				Frequency:   int(result.frequency),
				Programs:    programs,
			})
		}
	}
	log.Printf("Total channels found: %d\n", CountPrograms(channels))

	return channels, nil
}

package device

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
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

	id int

	clients []*io.PipeWriter

	InUse   bool
	Channel string
	Program string

	port int

	Name    string
	Model   string
	Address string
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

type DiscoverJson struct {
	FriendlyName    string
	ModelNumber     string
	FirmwareName    string
	FirmwareVersion string
	DeviceID        string
	BaseURL         string
}

func GetDiscoverJson(baseUrl string) DiscoverJson {
	cli := http.Client{
		Timeout: time.Second * 2,
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/discover.json", baseUrl), nil)
	if err != nil {
		log.Printf("error making request to discover endpoint: %v\n", err)
	}

	res, err := cli.Do(req)
	if err != nil {
		log.Printf("error making request to discover endpoint: %v\n", err)
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Printf("error reading discover endpoint: %v\n", err)
	}

	discover := DiscoverJson{}
	err = json.Unmarshal(body, &discover)
	if err != nil {
		log.Printf("error marshalling discover endpoint: %v\n", err)
	}

	return discover
}

func FindDevices(tunerPort int) []Device {
	wildcard, _ := strconv.ParseInt("0xFFFFFFFF", 0, 64)
	ptr := C.malloc(C.sizeof_struct_hdhomerun_discover_device_t)
	defer C.free(unsafe.Pointer(ptr))

	var discovered *C.struct_hdhomerun_discover_device_t
	var numFound C.int

	for {
		discovered = (*C.struct_hdhomerun_discover_device_t)(ptr)
		//TODO numFound can return -1 if the library has errors creating a socket
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

	log.Printf("Found %d HDHR device: %X %s (tuners: %d)\n", numFound, discovered.device_id, inet_ntoa((uint32)(discovered.ip_addr)), discovered.tuner_count)

	baseUrl := C.GoString(&discovered.base_url[0])
	discoverJson := GetDiscoverJson(baseUrl)

	devices := []Device{}
	for i := 0; i < int(discovered.tuner_count); i++ {
		device := Device{}
		device.hdhrDevice = C.hdhomerun_device_create(discovered.device_id, discovered.ip_addr, C.uint(i), nil)
		device.port = tunerPort + i
		device.Name = discoverJson.FriendlyName
		device.Model = discoverJson.ModelNumber
		device.Address = discoverJson.BaseURL

		devices = append(devices, device)
	}

	return devices
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

func (device *Device) GetStream(channel string, program string, target string) (*io.PipeReader, error) {
	if device.InUse {
		if channel == device.Channel && program == device.Program {
			r, w := io.Pipe()
			device.clients = append(device.clients, w)
			log.Printf("Adding client to stream %s/%s [clients: %d]\n", channel, program, len(device.clients))
			return r, nil
		}
		return nil, errors.New("device in use")
	}

	device.InUse = true
	device.Channel = channel
	device.Program = program

	r, w := io.Pipe()
	device.clients = append(device.clients, w)

	go device.streamThread()

	err := device.SetChannel(channel, program, target)
	if err != nil {
		log.Printf("error setting channel: %v", err)
	}

	return r, nil
}

func (device *Device) streamThread() {
	// allocate and listen on port
	// set channel/program/target
	// for loop copying bytes from udp to all channels
	addr := net.UDPAddr{
		Port: device.port,
		IP:   net.ParseIP("0.0.0.0"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	deadline := time.Now().Add(5 * time.Second)
	conn.SetReadDeadline(deadline)
	if err != nil {
		log.Println("Error listening:", err.Error())
		return
	}
	defer func() {
		conn.Close()
		device.InUse = false
	}()
	rconn := bufio.NewReader(conn)
	log.Printf("Stream thread started, listening on UDP :%d\n", device.port)

	buffer := make([]byte, 1500)

	for {
		if len(device.clients) == 0 {
			log.Println("No more stream clients, stopping stream")
			return
		}

		_, err := io.ReadFull(rconn, buffer)
		if err != nil {
			log.Println("error reading from UDP:", err.Error())
			return
		}
		conn.SetReadDeadline(time.Time{})

		for i := 0; i < len(device.clients); i++ {
			r := bytes.NewReader(buffer)
			_, err := io.Copy(device.clients[i], r)
			if err != nil && errors.Is(err, io.ErrClosedPipe) {
				device.clients = append(device.clients[:i], device.clients[i+1:]...)
				log.Printf("Client closed connection [clients: %d]\n", len(device.clients))
			}
		}
	}
}

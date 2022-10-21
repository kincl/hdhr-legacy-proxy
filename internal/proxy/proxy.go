package proxy

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"unsafe"

	"github.com/julienschmidt/httprouter"
)

// #cgo CFLAGS: -I../../libhdhomerun
// #cgo LDFLAGS: -lpthread -L../../libhdhomerun -lhdhomerun
// #include <stdio.h>
// #include <stdbool.h>
// #include <stdint.h>
// #include "hdhomerun.h"
import "C"

// inet_ntoa came from https://go.dev/play/p/JlYJXZnUxl
func inet_ntoa(ipInt64 uint32) (ip netip.Addr) {
	ipArray := [4]byte{byte(ipInt64 >> 24), byte(ipInt64 >> 16), byte(ipInt64 >> 8), byte(ipInt64)}
	ip = netip.AddrFrom4(ipArray)
	return
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

type Proxy struct {
	device    *C.struct_hdhomerun_device_t
	Hostname  string
	Port      string
	TunerPort string
	DataDir   string

	listings []Listing
	channels []ChannelScan
}

func countPrograms(channels []ChannelScan) int {
	count := 0
	for i := 0; i < len(channels); i++ {
		count += len(channels[i].Programs)
	}
	return count
}

func NewProxy(hostname, port, tunerPort, dataDir string) *Proxy {
	proxy := &Proxy{
		Hostname:  hostname,
		Port:      port,
		TunerPort: tunerPort,
		DataDir:   dataDir,
	}

	proxy.findDevices()

	if _, err := os.Stat(filepath.Join(proxy.DataDir, "channels.json")); err == nil {
		log.Printf("Parsing existing %s\n", filepath.Join(proxy.DataDir, "channels.json"))

		file, err := os.Open(filepath.Join(proxy.DataDir, "channels.json"))
		if err != nil {
			log.Fatal(err)
		}

		jb, err := io.ReadAll(file)
		if err != nil {
			log.Fatal(err)
		}

		var channels []ChannelScan
		err = json.Unmarshal(jb, &channels)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Loaded %d channels\n", countPrograms(channels))
		proxy.channels = channels

	} else if errors.Is(err, os.ErrNotExist) {
		log.Printf("No %s found, doing channel scan\n", filepath.Join(proxy.DataDir, "channels.json"))
		proxy.scan()

	} else {
		log.Fatal(err)
	}

	return proxy
}

func (proxy *Proxy) findDevices() {
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

	log.Printf("Found %d HDHR device: %X %s\n", numFound, discovered.device_id, inet_ntoa((uint32)(discovered.ip_addr)))
	proxy.device = C.hdhomerun_device_create(discovered.device_id, discovered.ip_addr, C.uint(0), nil)
}

// TODO fix this to separate out the stream from the http connection
func (proxy *Proxy) stream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)

	tunerPort, _ := strconv.Atoi(proxy.TunerPort)
	addr := net.UDPAddr{
		Port: tunerPort,
		IP:   net.ParseIP("0.0.0.0"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	deadline := time.Now().Add(5 * time.Second)
	conn.SetReadDeadline(deadline)
	if err != nil {
		log.Println("Error listening:", err.Error())
		http.Error(w, "Unable to allocate port", http.StatusFailedDependency)
		return
	}
	defer conn.Close()
	rconn := bufio.NewReader(conn)
	log.Printf("Connection opened, listening on UDP :%s\n", proxy.TunerPort)

	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("expected http.ResponseWriter to be an http.Flusher")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")

	channel := C.CString(fmt.Sprintf("auto:%s", ps.ByName("channel")))
	channel_ok := C.hdhomerun_device_set_tuner_channel(proxy.device, channel)
	C.free(unsafe.Pointer(channel))
	if channel_ok == 0 {
		log.Println("Unable to set tuner channel!")
	}

	program := C.CString(ps.ByName("program"))
	program_ok := C.hdhomerun_device_set_tuner_program(proxy.device, program)
	C.free(unsafe.Pointer(program))
	if program_ok == 0 {
		log.Println("Unable to set tuner program!")
	}

	target := C.CString(fmt.Sprintf("udp://%s:%s", proxy.Hostname, proxy.TunerPort))
	target_ok := C.hdhomerun_device_set_tuner_target(proxy.device, target)
	C.free(unsafe.Pointer(target))
	if target_ok == 0 {
		log.Println("Unable to set tuner target!")
	}

	for {
		select {
		case <-r.Context().Done():
			log.Printf("Connection closed, releasing UDP :%s\n", proxy.TunerPort)
			return
		default:
			_, err := io.CopyN(w, rconn, 1500)
			if err != nil {
				log.Println("error reading from UDP:", err.Error())
				return
			}
			conn.SetReadDeadline(time.Time{})
			flusher.Flush() // Trigger "chunked" encoding and send a chunk
		}
	}
}

// TODO needs error detection
func (proxy *Proxy) scan() {
	channelList := C.hdhomerun_channel_list_create(C.CString("us-bcast"))
	totalNum := (int)(C.hdhomerun_channel_list_total_count(channelList))

	channelScan := C.channelscan_create(proxy.device, C.CString("us-bcast"))

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

			proxy.channels = append(proxy.channels, ChannelScan{
				Channel_str: channel,
				Frequency:   int(result.frequency),
				Programs:    programs,
			})
		}
	}
	log.Printf("Total channels found: %d\n", countPrograms(proxy.channels))

	file, err := os.Create(filepath.Join(proxy.DataDir, "channels.json"))
	if err != nil {
		log.Fatal(err)
	}

	jb, err := json.Marshal(proxy.channels)
	if err != nil {
		log.Fatal(err)
	}

	_, err = file.Write(jb)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Wrote new %s\n", filepath.Join(proxy.DataDir, "channels.json"))
}

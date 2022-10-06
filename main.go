package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"
	"unsafe"

	"github.com/julienschmidt/httprouter"
)

// #cgo CFLAGS: -I/Users/jasonkincl/Workspace/libhdhomerun
// #cgo LDFLAGS: -lpthread -L/Users/jasonkincl/Workspace/libhdhomerun -lhdhomerun
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

type Listing struct {
	GuideNumber string
	GuideName   string
	URL         string
}

type Proxy struct {
	device    *C.struct_hdhomerun_device_t
	hostname  string
	port      string
	tunerPort string

	listings []Listing
}

func (proxy *Proxy) discover(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	discover := struct {
		FriendlyName    string
		ModelNumber     string
		FirmwareName    string
		TunerCount      int
		FirmwareVersion string
		DeviceID        string
		DeviceAuth      string
		BaseURL         string
		LineupURL       string
	}{
		FriendlyName:    "hdhrLegacyProxy",
		ModelNumber:     "HDTC-2US",
		FirmwareName:    "hdhomeruntc_atsc",
		TunerCount:      1,
		FirmwareVersion: "20150826",
		DeviceID:        "12345678",
		DeviceAuth:      "test1234",
		BaseURL:         fmt.Sprintf("http://%s:%s", proxy.hostname, proxy.port),
		LineupURL:       fmt.Sprintf("http://%s:%s/lineup.json", proxy.hostname, proxy.port),
	}
	json.NewEncoder(w).Encode(discover)
}

func (proxy *Proxy) lineupStatus(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	status := struct {
		ScanInProgress int
		ScanPossible   int
		Source         string
		SourceList     []string
	}{
		ScanInProgress: 0,
		ScanPossible:   1,
		Source:         "Antenna",
		SourceList:     []string{"Antenna"},
	}
	json.NewEncoder(w).Encode(status)
}

func (proxy *Proxy) lineup(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	json.NewEncoder(w).Encode(proxy.listings)
}

func (proxy *Proxy) stream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)

	tunerPort, _ := strconv.Atoi(proxy.tunerPort)
	addr := net.UDPAddr{
		Port: tunerPort,
		IP:   net.ParseIP("0.0.0.0"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Println("Error listening:", err.Error())
		http.Error(w, "Unable to allocate port", http.StatusFailedDependency)
		return
	}
	defer conn.Close()
	rconn := bufio.NewReader(conn)
	log.Printf("Connection opened, listening on UDP :%s\n", proxy.tunerPort)

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

	target := C.CString(fmt.Sprintf("udp://%s:%s", proxy.hostname, proxy.tunerPort))
	target_ok := C.hdhomerun_device_set_tuner_target(proxy.device, target)
	C.free(unsafe.Pointer(target))
	if target_ok == 0 {
		log.Println("Unable to set tuner target!")
	}

	for {
		select {
		case <-r.Context().Done():
			log.Printf("Connection closed, releasing UDP :%s\n", proxy.tunerPort)
			return
		default:
			if err != nil {
				log.Println("error reading from UDP:", err.Error())
			}
			io.CopyN(w, rconn, 1500)
			flusher.Flush() // Trigger "chunked" encoding and send a chunk
		}
	}
}

func (proxy *Proxy) scan(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// ptr := C.malloc(C.sizeof_struct_hdhomerun_channelscan_t)
	// defer C.free(unsafe.Pointer(ptr))

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
			log.Printf("Found %s\n", C.GoString(&result.channel_str[0]))
			for j := 0; j < int(result.program_count); j++ {
				proxy.listings = append(proxy.listings, Listing{
					GuideNumber: C.GoString(&result.programs[j].program_str[0]),
					GuideName:   C.GoString(&result.programs[j].name[0]),
					URL: fmt.Sprintf("http://%s:%s/auto/%d/%d",
						proxy.hostname,
						proxy.port,
						result.frequency,
						(int)(result.programs[j].program_number)),
				})
			}
		}
	}
	w.Write([]byte(fmt.Sprintf("listings found: %d\n", len(proxy.listings))))
}

func main() {
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
	device := C.hdhomerun_device_create(discovered.device_id, discovered.ip_addr, C.uint(0), nil)

	proxy := Proxy{
		device:    device,
		hostname:  "192.168.5.111",
		port:      "8000",
		tunerPort: "6000",
		listings:  []Listing{},
	}

	router := httprouter.New()
	router.GET("/discover.json", proxy.discover)
	router.GET("/lineup_status.json", proxy.lineupStatus)
	router.GET("/lineup.json", proxy.lineup)
	router.GET("/auto/:channel/:program", proxy.stream)
	router.GET("/scan", proxy.scan)

	log.Printf("Listening on :%s\n", proxy.port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", proxy.port), router))
}

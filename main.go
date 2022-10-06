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

type Proxy struct {
	device *C.struct_hdhomerun_device_t
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
		BaseURL:         "proxy_url",
		LineupURL:       "{proxy_url}/lineup.json",
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
}

func (proxy *Proxy) stream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)

	addr := net.UDPAddr{
		Port: 6000,
		IP:   net.ParseIP("0.0.0.0"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		http.Error(w, "Unable to allocate port", http.StatusFailedDependency)
		return
	}
	defer conn.Close()
	rconn := bufio.NewReader(conn)
	fmt.Println("Connection opened, listening on UDP :6000")

	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("expected http.ResponseWriter to be an http.Flusher")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")

	channel := C.CString(fmt.Sprintf("auto:%s", ps.ByName("channel")))
	channel_ok := C.hdhomerun_device_set_tuner_channel(proxy.device, channel)
	C.free(unsafe.Pointer(channel))
	if channel_ok == 0 {
		fmt.Println("Unable to set tuner channel!")
	}

	program := C.CString(ps.ByName("program"))
	program_ok := C.hdhomerun_device_set_tuner_program(proxy.device, program)
	C.free(unsafe.Pointer(program))
	if program_ok == 0 {
		fmt.Println("Unable to set tuner program!")
	}

	target := C.CString("udp://192.168.5.111:6000")
	target_ok := C.hdhomerun_device_set_tuner_target(proxy.device, target)
	C.free(unsafe.Pointer(target))
	if target_ok == 0 {
		fmt.Println("Unable to set tuner target!")
	}

	for {
		select {
		case <-r.Context().Done():
			fmt.Println("Connection closed, releasing UDP :6000")
			return
		default:
			if err != nil {
				fmt.Println("error reading from UDP:", err.Error())
			}
			io.CopyN(w, rconn, 1500)
			flusher.Flush() // Trigger "chunked" encoding and send a chunk
		}
	}
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

		fmt.Println("Did not find any devices! Trying again in 1s...")
		time.Sleep(time.Second * 1)
	}

	fmt.Printf("Found %d HDHR device: %X %s\n", numFound, discovered.device_id, inet_ntoa((uint32)(discovered.ip_addr)))
	device := C.hdhomerun_device_create(discovered.device_id, discovered.ip_addr, C.uint(0), nil)

	proxy := Proxy{device: device}

	router := httprouter.New()
	router.GET("/discover.json", proxy.discover)
	router.GET("/lineup_status.json", proxy.lineupStatus)
	router.GET("/lineup.json", proxy.lineup)
	router.GET("/auto/:channel/:program", proxy.stream)

	log.Print("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

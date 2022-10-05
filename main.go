package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"
	"unsafe"
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

func main() {
	wildcard, _ := strconv.ParseInt("0xFFFFFFFF", 0, 64)
	ptr := C.malloc(C.sizeof_struct_hdhomerun_discover_device_t)
	defer C.free(unsafe.Pointer(ptr))

discover:
	discovered := (*C.struct_hdhomerun_discover_device_t)(ptr)
	numFound := C.hdhomerun_discover_find_devices_custom_v2(
		C.uint(0),
		C.uint(wildcard),
		C.uint(wildcard),
		discovered,
		1)

	if numFound == 0 {
		fmt.Println("Did not find any devices! Sleeping for 1s...")
		time.Sleep(time.Second * 1)
		goto discover
	}

	fmt.Printf("Found %d HDHR device: %X %s\n", numFound, discovered.device_id, inet_ntoa((uint32)(discovered.ip_addr)))
	device := C.hdhomerun_device_create(discovered.device_id, discovered.ip_addr, C.uint(0), nil)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

		channel := C.CString("auto:29")
		channel_ok := C.hdhomerun_device_set_tuner_channel(device, channel)
		C.free(unsafe.Pointer(channel))
		if channel_ok == 0 {
			fmt.Println("Unable to set tuner channel!")
		}

		program := C.CString("3")
		program_ok := C.hdhomerun_device_set_tuner_program(device, program)
		C.free(unsafe.Pointer(program))
		if program_ok == 0 {
			fmt.Println("Unable to set tuner program!")
		}

		target := C.CString("udp://192.168.5.111:6000")
		target_ok := C.hdhomerun_device_set_tuner_target(device, target)
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
	})

	log.Print("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

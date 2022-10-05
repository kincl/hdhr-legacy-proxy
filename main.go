package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
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

	numFound := C.hdhomerun_discover_find_devices_custom_v2(
		C.uint(0),
		C.uint(wildcard),
		C.uint(wildcard),
		(*C.struct_hdhomerun_discover_device_t)(ptr),
		1)

	if numFound == 0 {
		fmt.Println("Did not find any devices! Exiting")
		os.Exit(1)
	}
	device := (*C.struct_hdhomerun_discover_device_t)(ptr)
	fmt.Printf("Found %d HDHR device: %X %s\n", numFound, device.device_id, inet_ntoa((uint32)(device.ip_addr)))

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

		flusher, ok := w.(http.Flusher)
		if !ok {
			panic("expected http.ResponseWriter to be an http.Flusher")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")

		fmt.Println("Connection opened, listening on UDP :6000")

		cmd := exec.Command("/bin/bash", "test_tv.sh") // TODO
		e := cmd.Run()
		if e != nil {
			fmt.Println("error running:", e.Error())
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

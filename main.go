package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os/exec"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// go func() {
		// 	<-r.Context().Done()
		// 	fmt.Println("Done")
		// }()
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

		// var buf [1358]byte
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
				// fmt.Fprintf(w, "Chunk #%d\n", i)
				// _, _, err := conn.ReadFromUDP(buf[:])
				if err != nil {
					fmt.Println("error reading from UDP:", err.Error())
				}
				// fmt.Println("read bytes:", num)
				// w.Write(buf[:])
				io.CopyN(w, rconn, 1500)

				flusher.Flush() // Trigger "chunked" encoding and send a chunk...
				// time.Sleep(500 * time.Millisecond)
			}
		}

	})

	log.Print("Listening on localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

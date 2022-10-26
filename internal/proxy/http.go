package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
)

type Listing struct {
	GuideNumber string
	GuideName   string
	URL         string
}

func (proxy *Proxy) GetRouter() *httprouter.Router {
	router := httprouter.New()
	router.GET("/", proxy.index)
	router.GET("/discover.json", proxy.discover)
	router.GET("/lineup_status.json", proxy.lineupStatus)
	router.GET("/lineup.json", proxy.lineup)
	router.GET("/auto/:channel/:program", proxy.stream)
	router.GET("/scan", proxy.httpScan)

	return router
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
		BaseURL:         fmt.Sprintf("http://%s:%s", proxy.Hostname, proxy.Port),
		LineupURL:       fmt.Sprintf("http://%s:%s/lineup.json", proxy.Hostname, proxy.Port),
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
	var listings []Listing

	for i := 0; i < len(proxy.channels); i++ {
		for j := 0; j < len(proxy.channels[i].Programs); j++ {
			listings = append(listings, Listing{
				GuideNumber: strings.Split(proxy.channels[i].Programs[j].Program_str, " ")[1],
				GuideName:   proxy.channels[i].Programs[j].Name,
				URL: fmt.Sprintf("http://%s:%s/auto/%d/%d",
					proxy.Hostname,
					proxy.Port,
					proxy.channels[i].Frequency,
					proxy.channels[i].Programs[j].Program_number),
			})
		}
	}

	json.NewEncoder(w).Encode(listings)
}

func (proxy *Proxy) httpScan(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
	go proxy.ScanAndSaveDB()
	w.Write([]byte("Scan initiated\n"))
}

func (proxy *Proxy) index(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Write([]byte("TODO Work in Progress\n"))
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

	err = proxy.device.SetChannel(ps.ByName("channel"), ps.ByName("program"), fmt.Sprintf("%s:%s", proxy.Hostname, proxy.TunerPort))
	if err != nil {
		log.Printf("error setting channel: %v", err)
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

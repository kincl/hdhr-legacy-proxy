package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	"github.com/kincl/hdhr-legacy-proxy/internal/device"
	"github.com/kincl/hdhr-legacy-proxy/web"
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
	router.NotFound = notFoundHandler{}

	return router
}

type notFoundHandler struct {
}

func (h notFoundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Not Found: %s %s %s\n", r.RemoteAddr, r.Method, r.URL)
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
		TunerCount:      len(proxy.devices),
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

type Device struct {
	Name    string
	Model   string
	Address string
}

type Tuner struct {
	InUse   bool
	Channel string
	Program string
	Clients int
}

type Page struct {
	Device   Device
	Channels []device.ChannelScan
	Tuners   []Tuner
}

func (proxy *Proxy) lookupChannel(channel, program string) (string, string) {
	channelint, _ := strconv.Atoi(channel)
	programint, _ := strconv.Atoi(channel)
	for i := 0; i < len(proxy.channels); i++ {
		if channelint == proxy.channels[i].Frequency {
			for j := 0; j < len(proxy.channels[i].Programs); j++ {
				if programint == proxy.channels[i].Programs[j].Program_number {
					return proxy.channels[i].Channel_str, proxy.channels[i].Programs[j].Program_str
				}
			}
		}
	}
	return channel, program
}

func (proxy *Proxy) index(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	T, err := template.ParseFS(web.Content, "index.html")
	if err != nil {
		fmt.Println(err)
	}

	tuners := []Tuner{}
	for i := 0; i < len(proxy.devices); i++ {
		c, p := proxy.lookupChannel(proxy.devices[i].Channel, proxy.devices[i].Program)
		t := Tuner{
			InUse:   proxy.devices[i].InUse,
			Channel: c,
			Program: p,
			Clients: len(proxy.devices[i].Clients),
		}
		tuners = append(tuners, t)
	}

	page := Page{
		Channels: proxy.channels,
		Device: Device{
			Name:    proxy.devices[0].Name,
			Model:   proxy.devices[0].Model,
			Address: proxy.devices[0].Address,
		},
		Tuners: tuners,
	}

	err = T.Execute(w, page)
	if err != nil {
		fmt.Printf("error executing template: %v\n", err)
	}
}

func (proxy *Proxy) stream(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var stream *io.PipeReader
	var err error
	log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)

	flusher, ok := w.(http.Flusher)
	if !ok {
		panic("expected http.ResponseWriter to be an http.Flusher")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")

	for i := 0; i < len(proxy.devices); i++ {
		log.Printf("trying to stream from tuner %d\n", i)
		stream, err = proxy.devices[i].GetStream(
			ps.ByName("channel"),
			ps.ByName("program"),
			fmt.Sprintf("%s:%d", proxy.Hostname, proxy.devices[i].Port))

		if err == nil {
			break
		}
		if err != nil && !errors.Is(err, errors.New("device in use")) {
			log.Printf("stream error: %v", err)
		}
	}

	if stream == nil {
		log.Printf("unable to stream: %v", err)
		return
	}

	for {
		select {
		case <-r.Context().Done():
			// log.Printf("Connection closed, releasing UDP :%s\n", proxy.TunerPort)
			stream.Close()
			return
		default:
			io.Copy(w, stream)

			if err != nil {
				log.Println("error reading from UDP:", err.Error())
				return
			}
			flusher.Flush() // Trigger "chunked" encoding and send a chunk
		}
	}
}

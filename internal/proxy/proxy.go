package proxy

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/kincl/hdhr-legacy-proxy/internal/device"
)

type Proxy struct {
	device device.Device

	Hostname  string
	Port      string
	TunerPort string
	DataDir   string

	channels []device.ChannelScan
}

func NewProxy(hostname, port, tunerPort, dataDir string) *Proxy {
	proxy := &Proxy{
		Hostname:  hostname,
		Port:      port,
		TunerPort: tunerPort,
		DataDir:   dataDir,
	}

	proxy.device = device.Device{}
	proxy.device.FindDevices(tunerPort)

	err := proxy.loadDB()
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("No %s found, doing channel scan\n", filepath.Join(proxy.DataDir, "channels.json"))
		proxy.ScanAndSaveDB()
	} else if err != nil {
		log.Printf("error loading database: %v", err)
	}

	return proxy
}

func (proxy *Proxy) ScanAndSaveDB() {
	channels, err := proxy.device.Scan()
	if err != nil {
		log.Fatalf("error scanning for channels: %v", err)
	}
	proxy.channels = channels

	err = proxy.saveDB()
	if err != nil {
		log.Printf("error saving database: %v", err)
	}
}

func (proxy *Proxy) loadDB() error {
	_, err := os.Stat(filepath.Join(proxy.DataDir, "channels.json"))
	if err != nil {
		return err
	}

	log.Printf("Parsing existing %s\n", filepath.Join(proxy.DataDir, "channels.json"))
	file, err := os.Open(filepath.Join(proxy.DataDir, "channels.json"))
	if err != nil {
		log.Fatal(err)
	}

	jb, err := io.ReadAll(file)
	if err != nil {
		log.Fatal(err)
	}

	var channels []device.ChannelScan
	err = json.Unmarshal(jb, &channels)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Loaded %d channels\n", device.CountPrograms(channels))
	proxy.channels = channels
	return nil
}

func (proxy *Proxy) saveDB() error {
	file, err := os.Create(filepath.Join(proxy.DataDir, "channels.json"))
	if err != nil {
		log.Fatal(err)
		return err
	}

	jb, err := json.Marshal(proxy.channels)
	if err != nil {
		log.Fatal(err)
		return err
	}

	_, err = file.Write(jb)
	if err != nil {
		log.Fatal(err)
		return err
	}
	log.Printf("Wrote new %s\n", filepath.Join(proxy.DataDir, "channels.json"))
	return nil
}

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/kincl/hdhr-legacy-proxy/internal/proxy"
	"github.com/spf13/cobra"
)

var (
	hostname       string
	port           string
	tunerPort      string
	dataDir        string
	tunerIPAddress string
	tunerCount     int

	rootCmd = &cobra.Command{
		Use:   "hdhr-legacy-proxy",
		Short: "hdhr-legacy-proxy emulates a newer HDHomeRun device for applications like Plex",

		Run: func(cmd *cobra.Command, args []string) {
			proxy := proxy.NewProxy(hostname, port, tunerPort, dataDir)

			if tunerIPAddress != "" {
				log.Printf("Not doing device discovery, using %s with %d tuners\n", tunerIPAddress, tunerCount)
				proxy.CreateDevices(tunerIPAddress, tunerCount)

			} else {
				proxy.DiscoverDevices()
			}

			log.Printf("Listening on %s:%s\n", proxy.Hostname, proxy.Port)
			log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", proxy.Port), proxy.GetRouter()))
		},
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&port, "port", "p", "8000", "Frontend proxy listen port")
	rootCmd.PersistentFlags().StringVarP(&dataDir, "dataDir", "d", ".", "Data directory")

	rootCmd.PersistentFlags().StringVar(&hostname, "backendIP", "127.0.0.1", "Backend proxy listen IP address")
	rootCmd.PersistentFlags().StringVar(&tunerPort, "backendPort", "6000", "Backend proxy listen port")

	rootCmd.PersistentFlags().StringVar(&tunerIPAddress, "tunerIP", "", "Tuner IP address, no discovery")
	rootCmd.PersistentFlags().IntVar(&tunerCount, "tunerCount", 0, "Tuner count, required when specifying Tuner IP")
}

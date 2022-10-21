package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/kincl/hdhr-legacy-proxy/internal/proxy"
	"github.com/spf13/cobra"
)

var (
	hostname  string
	port      string
	tunerPort string
	dataDir   string

	rootCmd = &cobra.Command{
		Use:   "hdhr-legacy-proxy",
		Short: "hdhr-legacy-proxy emulates a newer HDHomeRun device for applications like Plex",

		Run: func(cmd *cobra.Command, args []string) {
			proxy := proxy.NewProxy(hostname, port, tunerPort, dataDir)

			log.Printf("Listening on %s:%s\n", proxy.Hostname, proxy.Port)
			log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", proxy.Port), proxy.GetRouter()))
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&hostname, "hostname", "127.0.0.1", "IP address of the proxy")
	rootCmd.PersistentFlags().StringVarP(&port, "port", "p", "8000", "Frontend proxy listen port")
	rootCmd.PersistentFlags().StringVar(&tunerPort, "tunerPort", "6000", "Backend proxy listen port")
	rootCmd.PersistentFlags().StringVarP(&dataDir, "dataDir", "d", ".", "Data directory")
}

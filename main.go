package main

import (
	"crypto/tls"
	"crypto/x509/pkix"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	cert "github.com/ron96G/go-common-utils/certificate"
	log "github.com/ron96G/go-common-utils/log"

	"github.com/ron96G/clamav-facade/api"
	"github.com/ron96G/clamav-facade/clamav"
	"github.com/ron96G/clamav-facade/cmd"
)

var (
	loglevel  = flag.String("loglevel", "info", "loglevel of the application")
	logformat = flag.String("logformat", "json", "logformat of the application")
	hostname  = flag.String("client.hostname", "localhost", "the hostname of clamd")
	port      = flag.Uint("client.port", 3310, "the port of clamd")
	timeout   = flag.Duration("client.timeout", time.Second*10, "clamd connection timeout")
	maxSize   = flag.Int("maxsize", 25, "file size limit in mb")

	startAPI     = flag.Bool("api", false, "start the API")
	timeoutRead  = flag.Duration("api.readtimeout", time.Second*15, "http server timeout for reading request (requires --api)")
	timeoutWrite = flag.Duration("api.writetimeout", time.Second*15, "http server timeout for writing response (requires --api)")
	address      = flag.String("api.addr", "0.0.0.0:8080", "the address of the API (requires --api)")
	prefix       = flag.String("api.prefix", "", "the prefix of the API (requires --api)")
	enableTLS    = flag.Bool("api.tls", false, "enable TLS on the API (requires --api)")
	pemFile      = flag.String("pem", "", "PEM file for server TLS. If empty, a self-signed is generated")
	p12File      = flag.String("p12", "", "P12 file for server TLS. Use 'P12_PASSWORD' to provide the password. If empty, a self-signed is generated")
)

func main() {

	flag.Parse()
	log.Reset()
	log.Configure(*loglevel, *logformat, os.Stdout)

	client, err := clamav.NewClamavClient(*hostname, *port, time.Duration(*timeout)*time.Second)
	if err != nil {
		log.Error("failed to create new clamav client", "error", err.Error())
	}
	client.MaxSize = *maxSize * 1024 * 1024
	client.Log = log.New("client_logger")

	// API config
	if *startAPI {
		var tlsCfg *tls.Config
		if *enableTLS {
			tlsCfg, err = cert.GetServerTLS(cert.Options{
				PemFile:  *pemFile,
				P12File:  *p12File,
				Password: os.Getenv("P12_PASSWORD"),
				Subject: pkix.Name{
					Organization: []string{"DMC Virusscanner Facade"},
					Country:      []string{"DE"},
					Province:     []string{"NRW"},
					Locality:     []string{"Bonn"},
				},
			})
			if err != nil {
				log.Error("failed to setup tls config", "error", err)
			}
		}

		stopChan := SetupSignalHandler()
		api := api.NewAPI(*prefix, *address, client, stopChan, log.New("api_logger"), tlsCfg)
		api.ReadTimeout = *timeoutRead
		api.WriteTimeout = *timeoutWrite
		api.Run()

	} else {
		// commands
		cmd.Run(client, log.New("cmd_logger"))
	}
}

func SetupSignalHandler() (stopCh <-chan struct{}) {
	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(143) // second signal. Exit directly.
	}()

	return stop
}

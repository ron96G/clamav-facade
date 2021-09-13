package cmd

import (
	"flag"
	"os"

	"github.com/ron96G/clamav-facade/api"
	log "github.com/ron96G/go-common-utils/log"
)

var (
	file     = flag.String("file", "", "the file which will be scanned")
	reload   = flag.Bool("reload", false, "reload clamd")
	ping     = flag.Bool("ping", true, "ping clamd")
	stats    = flag.Bool("stats", false, "get stats about the scan queue")
	version  = flag.Bool("version", false, "ping clamd")
	shutdown = flag.Bool("shutdown", false, "shutdown clamd")
)

func Run(client api.Client, logger log.Logger) {
	if *ping {
		ok := client.Ping()
		if !ok {
			logger.Error("failed to ping clamav")
			os.Exit(1)
		}
	}

	if *version {
		version, err := client.Version()
		if err != nil {
			logger.Error("failed to get version of clamav", "error", err)
			os.Exit(1)
		}
		logger.Info(version)
	}

	if *stats {
		stats, err := client.Stats()
		if err != nil {
			logger.Error("failed to get stats of clamav", "error", err)
			os.Exit(1)
		}
		logger.Info(stats)
	}

	if *reload {
		err := client.Reload()
		if err != nil {
			logger.Error("failed to reload clamav", "error", err)
			os.Exit(1)
		}
	}

	if *file != "" {
		var ok bool
		var err error

		ok, err = client.ScanFile(*file)
		if err != nil {
			logger.Error("failed to scan file", "error", err)
			os.Exit(1)
		}

		if !ok {
			logger.Warn("virus found", "file", *file)
			os.Exit(1)
		}
		logger.Info("successfully scanned file", "file", *file)
	}

	if *shutdown {
		client.Shutdown()
	}

}

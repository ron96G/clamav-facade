package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/ron96G/clamav-facade/api"
	"github.com/sirupsen/logrus"
)

var (
	file     = flag.String("file", "", "the file which will be scanned")
	reload   = flag.Bool("reload", false, "reload clamd")
	ping     = flag.Bool("ping", true, "ping clamd")
	stats    = flag.Bool("stats", false, "get stats about the scan queue")
	version  = flag.Bool("version", false, "ping clamd")
	shutdown = flag.Bool("shutdown", false, "shutdown clamd")
)

func Run(client api.Client, logger *logrus.Logger) {
	if *ping {
		ok := client.Ping()
		if !ok {
			logger.Fatal(fmt.Errorf("failed to ping"))
		}
	}

	if *version {
		version, err := client.Version()
		if err != nil {
			logger.Fatal(fmt.Errorf("failed to get version"))
		}
		logger.Info(version)
	}

	if *stats {
		stats, err := client.Stats()
		if err != nil {
			logger.Fatal(fmt.Errorf("failed to get stats"))
		}
		logger.Info(stats)
	}

	if *reload {
		err := client.Reload()
		if err != nil {
			logger.Fatal(fmt.Errorf("failed to reload"))
		}
	}

	if *file != "" {
		var ok bool
		var err error

		ok, err = client.ScanFile(*file)
		if err != nil {
			logger.Fatal(err)
		}

		if !ok {
			if err != nil {
				logger.Error(err)
				os.Exit(1)
			}
			logger.Warnf("'%s' contains a virus", *file)
			os.Exit(1)
		}
		logger.Infof("successfully scanned file '%s'", *file)
	}

	if *shutdown {
		client.Shutdown()
	}

}

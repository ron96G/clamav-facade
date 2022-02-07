package cmd

import (
	"context"
	"flag"
	"os"
	"time"

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
	ctx := context.Background()

	if *ping {
		ok, err := client.Ping(ctx)
		if !ok {
			if err != nil {
				logger.Error("failed to ping clamav", "error", err)
				os.Exit(1)
			}
			logger.Error("failed to ping clamav but no error")
			os.Exit(1)
		}
	}

	if *version {
		version, err := client.Version(ctx)
		if err != nil {
			logger.Error("failed to get version of clamav", "error", err)
			os.Exit(1)
		}
		logger.Info(version)
	}

	if *stats {
		stats, err := client.Stats(ctx)
		if err != nil {
			logger.Error("failed to get stats of clamav", "error", err)
			os.Exit(1)
		}
		logger.Info(stats)
	}

	if *reload {
		err := client.Reload(ctx)
		if err != nil {
			logger.Error("failed to reload clamav", "error", err)
			os.Exit(1)
		}
	}

	if *file != "" {
		var ok bool
		var err error
		start := time.Now()

		logger.Info("scanning file", "file", *file)

		ok, err = client.ScanFile(ctx, *file)
		if err != nil {
			logger.Error("failed to scan file", "error", err, "elapsed_time", time.Since(start))
			os.Exit(1)
		}

		if !ok {
			logger.Warn("virus found", "file", *file, "elapsed_time", time.Since(start))
			os.Exit(1)
		}
		logger.Info("successfully scanned file", "file", *file, "elapsed_time", time.Since(start))
	}

	if *shutdown {
		client.Shutdown(ctx)
	}

}

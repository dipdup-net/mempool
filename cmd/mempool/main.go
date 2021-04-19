package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/dipdup-net/mempool/internal/cmdline"
	"github.com/dipdup-net/mempool/internal/config"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	args := cmdline.Parse()
	if args.Help {
		return
	}

	cfg, err := config.LoadAndValidate(args.Config)
	if err != nil {
		log.Error(err)
		return
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	indexers := make(map[string]*Indexer)

	for network, mempool := range cfg.Mempool.Indexers {
		indexer, err := NewIndexer(network, mempool, cfg.Database, cfg.Mempool.Settings)
		if err != nil {
			log.Error(err)
			return
		}
		indexers[network] = indexer

		if err := indexer.Start(); err != nil {
			log.Error(err)
			return
		}
	}

	<-signals
	for _, indexer := range indexers {
		log.Warn("Trying carefully stopping....")
		if err := indexer.Close(); err != nil {
			log.Error(err)
			return
		}
	}

	close(signals)
}

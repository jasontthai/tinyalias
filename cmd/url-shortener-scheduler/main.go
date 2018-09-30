package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/heroku/x/hmetrics/onload"
	log "github.com/sirupsen/logrus"
	"github.com/zirius/url-shortener/modules/queue"
)

func init() {
	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)
}

func main() {

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("$DATABASE_URL must be set")
	}

	pgxpool, qc, err := queue.Setup(databaseURL)
	if err != nil {
		log.Fatal("error initializing que-go")
	}
	defer pgxpool.Close()

	// Channel for catching shutdown signal
	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGINT, syscall.SIGTERM)

	channel := make(chan bool, 1)

	go func() {
		channel <- true
	}()

loop:
	for {
		select {
		case sig := <-term:
			log.WithFields(log.Fields{
				"signal": sig,
			}).Info("Caught shutdown signal")
			break loop
		case <-channel:
			queue.DispatchDetectSpamJob(qc, "")
			go func() {
				time.Sleep(24 * time.Hour) // Run every 24 hours
				channel <- true
			}()
		}
	}
}

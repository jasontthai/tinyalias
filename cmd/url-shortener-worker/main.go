package main

import (
	"encoding/json"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bgentry/que-go"
	"github.com/google/safebrowsing"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/jmoiron/sqlx"
	"github.com/oschwald/geoip2-golang"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/zirius/url-shortener/models"
	"github.com/zirius/url-shortener/modules/queue"
	"github.com/zirius/url-shortener/pg"
)

var (
	reader *geoip2.Reader
	db     *sqlx.DB
)

func init() {
	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)
}

func RunParseGeoRequestJob(j *que.Job) error {
	var request queue.ParseGeoRequest
	if err := json.Unmarshal(j.Args, &request); err != nil {
		return errors.Wrap(err, "Unable to unmarshal job arguments into ParseGeoRequest: "+string(j.Args))
	}

	log.WithField("ParseGeoRequest", request).Info("Processing ParseGeoRequest!")

	ip := net.ParseIP(request.IP)
	slug := request.Slug
	record, err := reader.City(ip)
	if err != nil {
		log.WithFields(log.Fields{
			"ip":   ip,
			"slug": slug,
		}).WithError(err).Error("Error Getting Geo Info")
	}

	var state string
	if len(record.Subdivisions) != 0 {
		state = record.Subdivisions[0].Names["en"]
	}

	if err = pg.UpsertURLStat(db, &models.URLStat{
		Slug:    slug,
		Country: record.Country.Names["en"],
		State:   state,
		Counter: 1,
		Created: time.Now(),
	}); err != nil {
		log.WithFields(log.Fields{
			"ip":   ip,
			"slug": slug,
		}).WithError(err).Error("Error Saving Geo Info")
	}

	return nil
}

func RunDetectSpamJob(j *que.Job) error {
	sb, err := safebrowsing.NewSafeBrowser(safebrowsing.Config{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		return err
	}

	clauses := make(map[string]interface{})
	clauses["status"] = "active"
	urls, err := pg.GetURLs(db, clauses)
	if err != nil {
		return err
	}

	var urlStr []string
	for _, url := range urls {
		urlStr = append(urlStr, url.Url)
	}

	threats, err := sb.LookupURLs(urlStr)
	if err != nil {
		return err
	}

	for i, url := range urls {
		if len(threats[i]) > 0 {
			// Detected link as threat - only need to get the first threat type
			url.Status = threats[i][0].ThreatType.String()
			err = pg.UpdateURL(db, &url)
			if err != nil {
				log.WithError(err).Error("Error updating url")
			}
		}
	}

	return nil
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

	reader, err = geoip2.Open("static/GeoLite2-City.mmdb")
	if err != nil {
		log.Fatal("error initializing geoip2")
	}
	defer reader.Close()

	db, err = sqlx.Open("postgres", databaseURL)
	if err != nil {
		log.Fatal("error initializing postgres")
	}
	defer db.Close()

	wm := que.WorkMap{
		queue.ParseGeoRequestJob: RunParseGeoRequestJob,
		queue.DetectSpamJob:      RunDetectSpamJob,
	}

	// 1 worker go routine
	workers := que.NewWorkerPool(qc, wm, 1)

	// Catch signal so we can shutdown gracefully
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go workers.Start()

	// Wait for a signal
	sig := <-sigCh
	log.WithField("signal", sig).Info("Signal received. Shutting down.")

	workers.Shutdown()
}

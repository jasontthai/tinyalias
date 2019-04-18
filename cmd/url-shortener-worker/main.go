package main

import (
	"encoding/json"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bgentry/que-go"
	"github.com/google/safebrowsing"
	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/oschwald/geoip2-golang"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/zirius/tinyalias/models"
	"github.com/zirius/tinyalias/modules/queue"
	"github.com/zirius/tinyalias/pg"
)

var (
	reader *geoip2.Reader
	db     *sqlx.DB
	sb     *safebrowsing.SafeBrowser
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

	ips := strings.Split(request.IP, ",")
	for _, ip := range ips {
		slug := request.Slug
		record, err := reader.City(net.ParseIP(ip))
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
	}

	return nil
}

func RunDetectSpamJob(j *que.Job) error {
	var err error
	var request queue.DetectSpamRequest
	if err = json.Unmarshal(j.Args, &request); err != nil {
		return errors.Wrap(err, "Unable to unmarshal job arguments into ParseGeoRequest: "+string(j.Args))
	}

	log.Info("Run spam job on: ", request.URL)

	limit := uint64(20)
	offset := uint64(0)
	for ; ; offset += limit {

		log.WithField("limit", limit).WithField("offset", offset).Debug("Getting URLs")

		clauses := make(map[string]interface{})
		clauses["_limit"] = limit
		clauses["_offset"] = offset

		if request.URL != "" {
			clauses["url"] = request.URL
		}

		urls, err := pg.GetURLs(db, clauses)
		if err != nil {
			return err
		}

		if len(urls) == 0 {
			break
		}

		var urlStr []string
		for _, url := range urls {
			urlStr = append(urlStr, url.Url)
		}

		threats, err := sb.LookupURLs(urlStr)
		if err != nil {
			log.WithError(err).Error("Error looking up urls")
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
	}

	return nil
}

func RunExpirationJob(j *que.Job) error {
	log.Info("Running Expiration Job")
	_, err := db.Exec("UPDATE urls SET status = 'expired' WHERE expired IS NOT NULL AND expired < NOW()")
	if err != nil {
		return err
	}

	//log.Info("Running Delete Job")
	//_, err = db.Exec("DELETE from urls WHERE created < CURRENT_DATE - interval '3' day")
	//return err
	return nil
}

func RunRemovePendingJob(j *que.Job) error {
	log.Info("Running Remove Pending Job")
	_, err := db.Exec("DELETE FROM urls WHERE status = 'pending'")
	return err
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

	sb, err = safebrowsing.NewSafeBrowser(safebrowsing.Config{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
		DBPath: "safebrowsing_db",
	})
	if err != nil {
		log.Fatal("error initializing safe browser client")
	}
	defer sb.Close()

	wm := que.WorkMap{
		queue.ParseGeoRequestJob: RunParseGeoRequestJob,
		queue.DetectSpamJob:      RunDetectSpamJob,
		queue.ExpirationJob:      RunExpirationJob,
		queue.RemovePendingJob:   RunRemovePendingJob,
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

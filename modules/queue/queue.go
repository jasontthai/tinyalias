package queue

import (
	"encoding/json"

	"github.com/bgentry/que-go"
	"github.com/jackc/pgx"
	"github.com/pkg/errors"
)

const (
	ParseGeoRequestJob = "ParseGeoRequestJob"
	DetectSpamJob      = "DetectSpamJob"
	ExpirationJob      = "ExpirationJob"
	RemovePendingJob   = "RemovePendingJob"
)

type ParseGeoRequest struct {
	IP   string `json:"ip"`
	Slug string `json:"slug"`
}

type DetectSpamRequest struct {
	URL string `json:"url"`
}

func DispatchParseGeoRequestJob(qc *que.Client, request ParseGeoRequest) error {
	enc, err := json.Marshal(request)
	if err != nil {
		return errors.Wrap(err, "Marshalling the ParseGeoRequestJob")
	}

	j := que.Job{
		Type: ParseGeoRequestJob,
		Args: enc,
	}

	return errors.Wrap(qc.Enqueue(&j), "Enqueueing Job")
}

// DispatchDetectSpamJob dispatches a job to que-go to detect
// unsafe links. If url is empty, it will scan all urls
func DispatchDetectSpamJob(qc *que.Client, url string) error {
	request := DetectSpamRequest{url}
	enc, err := json.Marshal(request)
	if err != nil {
		return errors.Wrap(err, "Marshalling the ParseGeoRequestJob")
	}

	j := que.Job{
		Type: DetectSpamJob,
		Args: enc,
	}

	return errors.Wrap(qc.Enqueue(&j), "Enqueueing Job")
}

func DispatchExpirationJob(qc *que.Client) error {
	j := que.Job{
		Type: ExpirationJob,
		Args: nil,
	}
	return errors.Wrap(qc.Enqueue(&j), "Enqueueing Job")
}

func DispatchRemovePendingJob(qc *que.Client) error {
	j := que.Job{
		Type: RemovePendingJob,
		Args: nil,
	}
	return errors.Wrap(qc.Enqueue(&j), "Enqueueing Job")
}

// GetPgxPool based on the provided database URL
func GetPgxPool(dbURL string) (*pgx.ConnPool, error) {
	pgxcfg, err := pgx.ParseURI(dbURL)
	if err != nil {
		return nil, err
	}

	pgxpool, err := pgx.NewConnPool(pgx.ConnPoolConfig{
		ConnConfig:   pgxcfg,
		AfterConnect: que.PrepareStatements,
	})

	if err != nil {
		return nil, err
	}

	return pgxpool, nil
}

// Setup a *pgx.ConnPool and *que.Client
// This is here so that setup routines can easily be shared between web and
// workers
func Setup(dbURL string) (*pgx.ConnPool, *que.Client, error) {
	pgxpool, err := GetPgxPool(dbURL)
	if err != nil {
		return nil, nil, err
	}

	qc := que.NewClient(pgxpool)

	return pgxpool, qc, err
}

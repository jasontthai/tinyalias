package test

import "os"

func GetTestPgURL() string {
	database := os.Getenv("DATABASE_URL")
	if database == "" {
		database = "postgres://localhost:12345/postgres?sslmode=disable"
	}
	return database
}

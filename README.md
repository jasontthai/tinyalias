
# TinyAlias

A url shortener Go app, which can easily be deployed to Heroku/Dokku/Flynn.

Powers [https://tinyalias.com](https://tinyalias.com)

# Installation

This installation guide assume you have already have installed postgres and heroku-cli

* Populate these 2 sql scripts: [init.sql](https://github.com/jasontthai/tinyalias/blob/master/sql/init.sql) and [schema.sql](https://github.com/bgentry/que-go/blob/master/schema.sql)

* Create .env file in code dir with these values:
  * `DATABASE_URL` : postgres db uri
  * `APP_NAME` : app name e.g. `test-tinyalias`
  * `BASE_URL` : for local run use `localhost:5000/`
  * `GOOGLE_API_KEY` : used for Google safebrowsing API
  * `SESSION_AUTHENTICATION_KEY` : used to auth cookie field
  * `SESSION_ENCRYPTION_KEY` : used to encrypt cookie field

# Local Run

* ```go install ./cmd/... && heroku local```



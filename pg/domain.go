package pg

import (
	"database/sql"

	"github.com/Masterminds/squirrel"
	"github.com/jasontthai/tinyalias/models"
	"github.com/jmoiron/sqlx"
)

func GetDomains(db *sqlx.DB, clauses map[string]interface{}) ([]models.Domain, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Select("*").
		From("domains").OrderBy("created desc")

	if blacklist, ok := clauses["blacklist"].(bool); ok {
		sb = sb.Where(squirrel.Eq{"blacklist": blacklist})
	}

	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return nil, err
	}
	var domains []models.Domain

	if err := db.Select(&domains, sqlStr, args...); err != nil {
		return nil, err
	}
	return domains, nil
}

func GetDomain(db *sqlx.DB, host string) (*models.Domain, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Select("*").
		From("domains")
	if host != "" {
		sb = sb.Where(squirrel.Eq{"host": host})
	}

	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := db.Queryx(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		var domain models.Domain
		if err := rows.StructScan(&domain); err != nil {
			return nil, err
		}
		return &domain, nil
	}
	return nil, sql.ErrNoRows
}

func CreateDomain(db *sqlx.DB, domain *models.Domain) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Insert("urls").Columns("host, blacklist, properties, created, updated").
		Values(domain.Host, domain.Blacklist, domain.Properties, domain.Created, domain.Updated)
	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return err
	}

	if _, err = db.Exec(sqlStr, args...); err != nil {
		return err
	}
	return nil
}

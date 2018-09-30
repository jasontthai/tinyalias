package pg

import (
	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/zirius/url-shortener/models"
)

func GetURLAccess(db *sqlx.DB, clauses map[string]interface{}) ([]models.URLStat, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Select("*").
		From("url_stats").OrderBy("created desc")

	if slug, ok := clauses["slug"].(string); ok {
		sb = sb.Where(squirrel.Eq{"slug": slug})
	}

	if country, ok := clauses["country"].(string); ok {
		sb = sb.Where(squirrel.Eq{"country": country})
	}

	if city, ok := clauses["city"].(string); ok {
		sb = sb.Where(squirrel.Eq{"city": city})
	}

	if ip, ok := clauses["ip"].(string); ok {
		sb = sb.Where(squirrel.Eq{"ip": ip})
	}

	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return nil, err
	}

	var stats []models.URLStat

	if err := db.Select(&stats, sqlStr, args...); err != nil {
		return nil, err
	}
	return stats, nil
}

func UpsertURLStat(db *sqlx.DB, stat *models.URLStat) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Insert("url_stats").Columns("slug, country, city, ip, counter, properties, created, updated").Values(
		stat.Slug, stat.Country, stat.City, stat.IP, stat.Counter, stat.Properties, stat.Created, stat.Updated).
		Suffix(`ON CONFLICT ON CONSTRAINT url_stats_pkey DO UPDATE SET counter = url_stats.counter + 1, updated = NOW()`)

	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return err
	}

	if _, err = db.Exec(sqlStr, args...); err != nil {
		return err
	}
	return nil
}

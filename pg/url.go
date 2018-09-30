package pg

import (
	"database/sql"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/zirius/url-shortener/models"
)

func GetURL(db *sqlx.DB, longUrl, slug string) (*models.URL, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Select("url, slug, ip, counter, created, updated, access_ips, status").
		From("urls")
	if longUrl != "" {
		sb = sb.Where(squirrel.Eq{"url": longUrl})
	}
	if slug != "" {
		sb = sb.Where(squirrel.Eq{"slug": slug})
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
		var url models.URL
		var accessIPs []string
		if err := rows.Scan(&url.Url, &url.Slug, &url.IP, &url.Counter, &url.Created, &url.Updated, pq.Array(&accessIPs), &url.Status); err != nil {
			return nil, err
		}
		url.AccessIPs = accessIPs
		return &url, nil
	}
	return nil, sql.ErrNoRows
}

func GetURLs(db *sqlx.DB) ([]models.URL, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Select("url, slug, ip, counter, created, updated, access_ips, status").
		From("urls").OrderBy("created desc")
	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return nil, err
	}

	var urls []models.URL

	rows, err := db.Queryx(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var url models.URL
		var accessIPs []string
		if err := rows.Scan(&url.Url, &url.Slug, &url.IP, &url.Counter, &url.Created, &url.Updated, pq.Array(accessIPs), &url.Status); err != nil {
			return nil, err
		}
		url.AccessIPs = accessIPs
		urls = append(urls, url)
	}
	return urls, nil
}

func CreateURL(db *sqlx.DB, url *models.URL) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Insert("urls").Columns("url, slug, ip, counter, created, updated").Values(url.Url, url.Slug, url.IP, url.Counter, url.Created, url.Updated)
	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return err
	}

	if _, err = db.Exec(sqlStr, args...); err != nil {
		return err
	}
	return nil
}

func UpdateURL(db *sqlx.DB, url *models.URL) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	clauses := make(map[string]interface{})
	clauses["counter"] = url.Counter
	clauses["access_ips"] = pq.Array(url.AccessIPs)
	sb := psql.Update("urls").SetMap(clauses).Where(squirrel.Eq{"url": url.Url})
	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return err
	}

	if _, err = db.Exec(sqlStr, args...); err != nil {
		return err
	}
	return nil
}

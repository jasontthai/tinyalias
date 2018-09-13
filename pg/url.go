package pg

import (
	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/zirius/url-shortener/models"
)

func GetURL(db *sqlx.DB, longUrl, slug string) (*models.URL, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Select("url, slug, ip, created, updated").
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

	var url models.URL
	if err = db.Get(&url, sqlStr, args...); err != nil {
		return nil, err
	}
	return &url, nil
}

func CreateURL(db *sqlx.DB, url *models.URL) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Insert("urls").Columns("url, slug, ip, created, updated").Values(url.Url, url.Slug, url.IP, url.Created, url.Updated)
	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return err
	}

	if _, err = db.Exec(sqlStr, args...); err != nil {
		return err
	}
	return nil
}

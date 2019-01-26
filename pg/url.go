package pg

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	"github.com/zirius/tinyalias/models"
)

func GetURL(db *sqlx.DB, slug string) (*models.URL, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Select("*").
		From("urls").Where(squirrel.Eq{"slug": slug})

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
		if err := rows.StructScan(&url); err != nil {
			return nil, err
		}
		return &url, nil
	}
	return nil, sql.ErrNoRows
}

func GetURLs(db *sqlx.DB, clauses map[string]interface{}) ([]models.URL, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Select("*").
		From("urls")

	if slug, ok := clauses["slug"].(string); ok {
		sb = sb.Where(squirrel.Eq{"slug": slug})
	}

	if url, ok := clauses["url"].(string); ok {
		sb = sb.Where(squirrel.Eq{"url": url})
	}

	if status, ok := clauses["status"].(string); ok {
		sb = sb.Where(squirrel.Eq{"status": status})
	}

	if username, ok := clauses["username"].(string); ok {
		sb = sb.Where(squirrel.Eq{"username": username})
	}

	if limit, ok := clauses["_limit"].(uint64); ok {
		sb = sb.Limit(limit)
	}

	if offset, ok := clauses["_offset"].(uint64); ok {
		sb = sb.Offset(offset)
	}

	// search field
	if like, ok := clauses["_like"].(string); ok && like != "" {
		sb = sb.Where(fmt.Sprintf("(slug ilike '%v' OR url ilike '%v' OR username ilike '%v')", like, like, like))
	}

	if orderBy, ok := clauses["_order_by"].(string); ok && orderBy != "" {
		sb = sb.OrderBy(orderBy)
	} else {
		sb = sb.OrderBy("created desc")
	}

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
		if err := rows.StructScan(&url); err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}
	return urls, nil
}

func GetURLCount(db *sqlx.DB, clauses map[string]interface{}) (int, error) {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Select("count(*)").
		From("urls")

	if slug, ok := clauses["slug"].(string); ok {
		sb = sb.Where(squirrel.Eq{"slug": slug})
	}

	if url, ok := clauses["url"].(string); ok {
		sb = sb.Where(squirrel.Eq{"url": url})
	}

	if status, ok := clauses["status"].(string); ok {
		sb = sb.Where(squirrel.Eq{"status": status})
	}

	if username, ok := clauses["username"].(string); ok {
		sb = sb.Where(squirrel.Eq{"username": username})
	}

	// search field
	if like, ok := clauses["_like"].(string); ok && like != "" {
		sb = sb.Where(fmt.Sprintf("(slug ilike '%v' OR url ilike '%v' OR username ilike '%v')", like, like, like))
	}

	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return 0, err
	}

	var count int
	err = db.Get(&count, sqlStr, args...)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func CreateURL(db *sqlx.DB, url *models.URL) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Insert("urls").Columns("url, slug, ip, counter, created, updated, password, expired, mindful, username").
		Values(url.Url, url.Slug, url.IP, url.Counter, url.Created, url.Updated, url.Password, url.Expired, url.Mindful, url.Username)
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
	clauses["status"] = url.Status
	clauses["updated"] = time.Now()
	sb := psql.Update("urls").SetMap(clauses).Where(squirrel.Eq{"slug": url.Slug})
	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return err
	}

	if _, err = db.Exec(sqlStr, args...); err != nil {
		return err
	}
	return nil
}

func DeleteURL(db *sqlx.DB, longUrl, slug string) error {
	if longUrl == "" && slug == "" {
		return fmt.Errorf("missing required field")
	}
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Delete("urls")
	if longUrl != "" {
		sb = sb.Where(squirrel.Eq{"url": longUrl})
	}
	if slug != "" {
		sb = sb.Where(squirrel.Eq{"slug": slug})
	}
	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return err
	}
	if _, err = db.Exec(sqlStr, args...); err != nil {
		return err
	}
	return nil
}

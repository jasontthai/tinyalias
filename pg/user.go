package pg

import (
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jasontthai/tinyalias/models"
	"github.com/jmoiron/sqlx"
)

func GetUser(db *sqlx.DB, username string) (*models.User, error) {
	if username == "" {
		return nil, fmt.Errorf("username is empty")
	}
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Select("*").
		From("users").
		Where(squirrel.Eq{"username": username})

	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := db.Get(&user, sqlStr, args...); err != nil {
		return nil, err
	}
	return &user, nil
}

func CreateUser(db *sqlx.DB, user *models.User) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	sb := psql.Insert("users").Columns("username, role, password, properties, created, updated").
		Values(user.Username, user.Role, user.Password, user.Properties, user.Created, user.Updated)
	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return err
	}

	if _, err = db.Exec(sqlStr, args...); err != nil {
		return err
	}
	return nil
}

func UpdateUser(db *sqlx.DB, user *models.User) error {
	psql := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	clauses := make(map[string]interface{})
	clauses["password"] = user.Password
	clauses["status"] = user.Status
	clauses["updated"] = time.Now()
	sb := psql.Update("users").SetMap(clauses).Where(squirrel.Eq{"username": user.Username})
	sqlStr, args, err := sb.ToSql()
	if err != nil {
		return err
	}

	if _, err = db.Exec(sqlStr, args...); err != nil {
		return err
	}
	return nil
}

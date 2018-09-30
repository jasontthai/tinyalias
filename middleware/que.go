package middleware

import (
	"github.com/bgentry/que-go"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx"
)

func Que(pgxpool *pgx.ConnPool, qc *que.Client) gin.HandlerFunc {

	return func(c *gin.Context) {
		c.Set("PgxPool", pgxpool)
		c.Set("QueClient", qc)
		c.Next()
	}
}

func GetQue(c *gin.Context) (*pgx.ConnPool, *que.Client) {
	return c.Value("PgxPool").(*pgx.ConnPool), c.Value("QueClient").(*que.Client)
}

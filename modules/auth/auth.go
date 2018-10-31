package auth

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zirius/tinyalias/middleware"
	"github.com/zirius/tinyalias/models"
	"github.com/zirius/tinyalias/pg"
)

const SessionName = "My-Session"

func Login(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	db := middleware.GetDB(c)
	user, err := pg.GetUser(db, username)
	if err != nil && err != sql.ErrNoRows {
		c.Error(err)
		c.HTML(http.StatusInternalServerError, "auth.tmpl.html", gin.H{
			"error": "Something went wrong. Try again.",
		})
		return
	}
	if user == nil {
		c.HTML(http.StatusBadRequest, "auth.tmpl.html", gin.H{
			"error": "User does not exist.",
		})
		return
	}

	if user.Status != "active" {
		c.HTML(http.StatusBadRequest, "auth.tmpl.html", gin.H{
			"error": "User is no longer active.",
		})
		return
	}

	err = models.VerifyPassword(user.Password, password)
	if err != nil {
		c.HTML(http.StatusBadRequest, "auth.tmpl.html", gin.H{
			"error": "Wrong Password.",
		})
		return
	}

	sessionStore := middleware.GetSessionStore(c)
	session, err := sessionStore.Get(c.Request, SessionName)
	if err != nil {
		c.Error(err)
		c.HTML(http.StatusInternalServerError, "auth.tmpl.html", gin.H{
			"error": "Something went wrong. Try again.",
		})
		return
	}

	session.Values["username"] = username
	if err := session.Save(c.Request, c.Writer); err != nil {
		c.Error(err)
		c.HTML(http.StatusInternalServerError, "auth.tmpl.html", gin.H{
			"error": "Something went wrong. Try again.",
		})
		return
	}

	c.Redirect(http.StatusFound, "/")
}

func Logout(c *gin.Context) {
	sessionStore := middleware.GetSessionStore(c)
	session, err := sessionStore.Get(c.Request, SessionName)
	if err != nil {
		c.Error(err)
		c.Redirect(http.StatusFound, "/")
		return
	}

	session.Values["username"] = ""
	if err := session.Save(c.Request, c.Writer); err != nil {
		c.Error(err)
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.Redirect(http.StatusFound, "/")
}

func Register(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	confirmPassword := c.PostForm("confirm_password")

	if username == "" || password == "" || password != confirmPassword {
		c.HTML(http.StatusBadRequest, "auth.tmpl.html", gin.H{
			"error": "Invalid request. Try again.",
		})
		return
	}

	db := middleware.GetDB(c)

	user, err := pg.GetUser(db, username)
	if err != nil && err != sql.ErrNoRows {
		c.Error(err)
		c.HTML(http.StatusInternalServerError, "auth.tmpl.html", gin.H{
			"error": "Something went wrong. Try again.",
		})
		return
	}
	if user != nil {
		c.HTML(http.StatusBadRequest, "auth.tmpl.html", gin.H{
			"error": "User already exists.",
		})
		return
	}

	hashedPassword, err := models.TransformPassword(password)
	if err != nil {
		c.Error(err)
		c.HTML(http.StatusInternalServerError, "auth.tmpl.html", gin.H{
			"error": "Something went wrong. Try again.",
		})
		return
	}

	err = pg.CreateUser(db, &models.User{
		Username: username,
		Password: hashedPassword,
		Created:  time.Now(),
	})
	if err != nil {
		c.Error(err)
		c.HTML(http.StatusInternalServerError, "auth.tmpl.html", gin.H{
			"error": "Something went wrong. Try again.",
		})
		return
	}

	sessionStore := middleware.GetSessionStore(c)
	session, err := sessionStore.Get(c.Request, SessionName)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err,
		})
		return
	}

	session.Values["username"] = username
	if err := session.Save(c.Request, c.Writer); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err,
		})
		return
	}

	c.Redirect(http.StatusFound, "/")
}

func UpdatePassword(c *gin.Context) {
	user := GetAuthenticatedUser(c)
	username := user.Username
	if user == nil {
		c.HTML(http.StatusBadRequest, "auth.tmpl.html", gin.H{
			"error": "You have to be logged in.",
		})
		return
	}

	currentPassword := c.PostForm("current_password")
	newPassword := c.PostForm("new_password")
	confirmNewPassword := c.PostForm("confirm_new_password")

	if currentPassword == "" || newPassword == "" || newPassword != confirmNewPassword {
		c.HTML(http.StatusBadRequest, "auth.tmpl.html", gin.H{
			"error": "Invalid request. Try again.",
			"user":  username,
		})
		return
	}

	db := middleware.GetDB(c)

	user, err := pg.GetUser(db, username)
	if err != nil && err != sql.ErrNoRows {
		c.Error(err)
		c.HTML(http.StatusInternalServerError, "auth.tmpl.html", gin.H{
			"error": "Something went wrong. Try again.",
			"user":  username,
		})
		return
	}
	if user == nil {
		c.HTML(http.StatusBadRequest, "auth.tmpl.html", gin.H{
			"error": "User does not exist.",
			"user":  username,
		})
		return
	}

	err = models.VerifyPassword(user.Password, currentPassword)
	if err != nil {
		c.HTML(http.StatusBadRequest, "auth.tmpl.html", gin.H{
			"error": "Wrong Password.",
			"user":  username,
		})
		return
	}

	hashedPassword, err := models.TransformPassword(newPassword)
	if err != nil {
		c.Error(err)
		c.HTML(http.StatusInternalServerError, "auth.tmpl.html", gin.H{
			"error": "Something went wrong. Try again.",
			"user":  username,
		})
		return
	}
	// update password
	user.Password = hashedPassword

	err = pg.UpdateUser(db, user)
	if err != nil {
		c.Error(err)
		c.HTML(http.StatusInternalServerError, "auth.tmpl.html", gin.H{
			"error": "Something went wrong. Try again.",
			"user":  username,
		})
		return
	}

	c.HTML(http.StatusOK, "auth.tmpl.html", gin.H{
		"message": "Successfully updated password.",
		"user":    username,
	})
}

func GetAuthenticatedUser(c *gin.Context) *models.User {
	db := middleware.GetDB(c)
	sessionStore := middleware.GetSessionStore(c)
	session, err := sessionStore.Get(c.Request, SessionName)
	if err != nil {
		c.Error(err)
	}

	username, found := session.Values["username"].(string)
	if found && username != "" {
		user, err := pg.GetUser(db, username)
		if err != nil {
			c.Error(err)
		}
		return user
	}
	return nil
}

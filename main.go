package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/GetStream/stream-go"
	"github.com/gin-gonic/gin"
	"github.com/pborman/uuid"
	"database/sql"
	"strings"
)

type AccessToken struct {
	Token  string
	Expiry time.Time
}

type User struct {
	ID       int64
	UUID     string
	Email    string
	Username string
}

type UserToken struct {
	Token  string
	Expiry time.Time
}

var router *gin.Engine
var DB *sql.DB

// Stream.io variables
var client *getstream.Client
var globalFeed *getstream.FlatFeed

func main() {

	// database setup
	DB, err := sql.Open("sqlite3", "/tmp/getstream-mobile-backend.db")
	if err != nil {
		panic("failed to connect database")
	}
	defer DB.Close()

	// GetStream.io setup

	client, err = getstream.New(&getstream.Config{
		APIKey:      os.Getenv("STREAM_API_KEY"),
		APISecret:   os.Getenv("STREAM_API_SECRET"),
		AppID:       os.Getenv("STREAM_APP_ID"),
		Location:    os.Getenv("STREAM_REGION"),
	})
	if err != nil {
		panic("failed to connect to stream")
	}
	globalFeed, err = client.FlatFeed("user", "")
	if err != nil {
		panic("could not set global feed")
	}

	// gin routing

	gin.SetMode(gin.DebugMode)
	router = gin.Default()
	router.LoadHTMLGlob("templates/*")

	router.Static("/", "index.html")
	router.GET("/src", func(c *gin.Context) {
		// redirect to the repo, blog post, etc.
		c.Redirect(http.StatusTemporaryRedirect, "//github.com/GetStream")
	})
	router.GET("/blog", func(c *gin.Context) {
		// redirect to the repo, blog post, etc.
		c.Redirect(http.StatusTemporaryRedirect, "//getstream.io/blog")
	})
	router.GET("/healthcheck", func(c *gin.Context) {
		// redirect to the repo, blog post, etc.
		c.HTML(http.StatusOK, "healthcheck.html", gin.H{})
	})
	router.Static("/privacy", "privacy.html")
	router.Static("/termsofservice", "termsofservice.html")

	router.GET("/user/:username", func(c *gin.Context) {
		name := c.Param("username")
		// get user stream, send back in json format
		c.JSON(http.StatusOK, gin.H{
			"username": name,
		})
	})

	router.GET("/follow/:target", getFollow)

	router.GET("/login", getLogin)
	router.POST("/login", postLogin)
	router.GET("/register", getRegister)
	router.POST("/register", postRegister)
	router.POST("/upload", postPhotoUpload)

	// no more custom code under here
	log.Print("Listening on port 8080")
	router.Run()
}

func getFollow(c *gin.Context) {
	//sourceFeedName := c.Param("sourceName")
	//targetFeedName := c.Param("targetName")

	// validate that sourceUuid and targetUuid are valid
	// source follows target, pull 100 items into their feed

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
	})
}

func postPhotoUpload(c *gin.Context) {
	// create copy to be used inside the goroutine
	cCp := c.Copy()
	go func() { // handle upload in the background
		username := cCp.PostForm("username")
		file, header, err := c.Request.FormFile("upload")
		filename := header.Filename
		log.Println(header.Filename)
		out, err := os.Create("./tmp/" + filename + ".png")
		if err != nil {
			log.Fatal(err)
		}
		defer out.Close()
		_, err = io.Copy(out, file)
		if err != nil {
			log.Fatal(err)
		}

		// shrink image
		// push to S3, get URL
		// send image url, date, username to stream



		// note that you are using the copied context "cCp", IMPORTANT
		log.Println("Done! in path " + cCp.Request.URL.Path)

	}()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func getRegister(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{})
}

func postRegister(c *gin.Context) {
	var user *User

	email := c.PostForm("email")
	username := c.PostForm("username")

	var output []string

	if username == "" || email == "" {
		if username == "" {
			output = append(output, "Username cannot be blank")
		}
		if email == "" {
			output = append(output, "Email cannot be blank")
		}
	} else {
		log.Println("checking username uniqueness")
		rows, err := DB.Query(`
			SELECT u.id, u.uuid. u.email, u.username FROM users u WHERE u.username=? OR u.email=?
			LIMIT 1
			`, strings.ToLower(username), strings.ToLower(email))
		if err != nil {
			log.Println("error getting user details for username", username)
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		for rows.Next() {
			var userId int = -1
			err = rows.Scan(&userId)
			if userId > -1 {
				output = append(output, "Username or Email already used")
			}
		}
	}

	if len(output) > 0 {
		c.HTML(http.StatusOK, "register.html", gin.H{"errors": output})
		return
	}

	log.Println("saving new user in db")
	// save user details in the db
	stmt, err := DB.Prepare(`
			INSERT INTO users (uuid, email, username, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?)`,
	)
	if err != nil {
		panic(fmt.Sprintf("could create user insert statement, %v", err))
	}
	_, err = stmt.Exec(
		uuid.New(),
		user.Email,
		user.Username,
		time.Now(),
		time.Now(),
	)
	if err != nil {
		panic("failed to run user insert statement")
	}

	//affect, err := res.RowsAffected()
	//if err != nil {
	//	panic("failed to run rows affected after user insert")
	//}
	//log.Println("rows affected:", affect)
	//
	//user.ID, err = res.LastInsertId()
	//if err != nil {
	//	panic("failed to get last_insert_id")
	//}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func getLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{})
}

func postLogin(c *gin.Context) {
	var output []string
	var userUUID string

	email := c.PostForm("email")
	username := c.PostForm("username")

	if username == "" || email == "" {
		if username == "" {
			output = append(output, "Username cannot be blank")
		}
		if email == "" {
			output = append(output, "Email cannot be blank")
		}
	} else {
		rows, err := DB.Query(`
			SELECT u.UUID FROM users u WHERE u.username=? AND u.email=?
			LIMIT 1
			`, strings.ToLower(username), strings.ToLower(email))
		if err != nil {
			log.Println("error getting user details for username", username)
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		for rows.Next() {
			err = rows.Scan(
				&userUUID,
			)
			if err != nil {
				log.Println("error putting user details into struct")
				c.JSON(http.StatusInternalServerError, err.Error())
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"UUID": userUUID})
	return
}

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
	"database/sql"
	"strings"

	"github.com/GetStream/stream-go"

	"github.com/disintegration/imaging"

	"github.com/gin-gonic/gin"
	"github.com/pborman/uuid"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/s3")

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
var S3 string
const S3BucketName string = "getstream-example"

// Stream.io variables
var client *getstream.Client
var globalFeed *getstream.FlatFeed

func main() {

	// database setup
	_, err := sql.Open("sqlite3", "./getstream-mobile-backend.db")
	if err != nil {
		fmt.Println("failed to connect database")
	}
	//defer DB.Close()

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
	globalFeed, err = client.FlatFeed("user", "global")
	if err != nil {
		panic("could not set global feed")
	}

	// S3
	creds := credentials.NewSharedCredentials("/.credentials", "default")
	_, err = creds.Get()
	if err != nil {
		panic(err)
	}
	aws.DefaultConfig.Region = "us-east-1"
	config := &aws.Config{
		Region:           "",
		Endpoint:         "s3.amazonaws.com", // <-- forking important !
		S3ForcePathStyle: true, // <-- without these lines. All will fail! fork you aws!
		Credentials:      creds,
		LogLevel:         0, // <-- feel free to crank it up
	}

	S3 := s3.New(config)
	S3 = S3

	// gin routing

	gin.SetMode(gin.DebugMode)
	router = gin.Default()
	router.LoadHTMLGlob("templates/*")

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
	//router.Static("/", "index.html")
	router.GET("/", func(c *gin.Context) {
		// redirect to the repo, blog post, etc.
		//c.Redirect(http.StatusTemporaryRedirect, "//getstream.io/blog")
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
		})
	})
	log.Print("Listening on port 8080")
	router.Run(":3000")
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
	go func() {
		var photoFilename string
		var photoURL string

		// handle upload in the background
		userUUID := cCp.PostForm("uuid")
		_, header, err := c.Request.FormFile("upload")
		filename := header.Filename
		log.Println(header.Filename)
		out, err := os.Create("./tmp/" + filename + ".png")
		if err != nil {
			log.Fatal(err)
		}
		photoFilename = uuid.New() + ".png"
		_, err = io.Copy(out, photoFilename)
		if err != nil {
			log.Fatal(err)
		}
		out.Close()

		// shrink image
		dstImage := imaging.Resize(out, 1024, 768, imaging.Lanczos)

		// push to S3, get URL
		file, err := os.Open()
		photoURL = "https://dvqg2dogggmn6.cloudfront.net/images/stream_logo.svg"


		// send image url, date, username to stream

		now := time.Now()
		userFeed, err := client.FlatFeed("user", userUUID)
		if err != nil {
			fmt.Println(err)
		} else {
			_, err = globalFeed.AddActivity(&getstream.Activity{
				Verb:      "photo",
				ForeignID: uuid.New(),
				TimeStamp: &now,
				To:        []getstream.Feed{globalFeed, userFeed},
				Object:    getstream.FeedID(fmt.Sprintf("photo:%s", photoFilename)),
				Actor:     getstream.FeedID(fmt.Sprintf("user:%s", userUUID)),
				MetaData:  map[string]string{
					// add as many custom keys/values here as you like
					"photoUrl": fmt.Sprintf("message %d", photoURL),
				},
			})
			if err != nil {
				fmt.Println(err)
			}
		}
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

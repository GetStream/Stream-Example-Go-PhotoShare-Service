package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"database/sql"
	"bytes"

	"github.com/GetStream/stream-go"

	"github.com/disintegration/imaging"

	"github.com/gin-gonic/gin"
	"github.com/pborman/uuid"
	"gopkg.in/gorp.v1"
	_ "github.com/go-sql-driver/mysql"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/service/s3"
	//"github.com/aws/aws-sdk-go/aws/session"
	"github.com/jinzhu/gorm"
	"strings"
)

type User struct {
	gorm.Model
	UUID      string `gorm:"column:uuid" json:"uuid"`
	Username  string `gorm:"column:username" json:"username"`
	Email     string `gorm:"column:email" json:"email"`
}

type Photo struct {
	gorm.Model
	UserID    int  `gorm:"column:user_id,index" json:"user_id"`
	UUID      string `gorm:"column:uuid" json:"uuid"`
	URL       string `gorm:"column:url" json:"url"`
	Likes     int `gorm:"column:likes" json:"likes"`
}

var router *gin.Engine
var S3Client *s3.S3
var S3BucketName string = "getstream-example"

type FeedItem struct {
	ID          string `json:"id"`
	AuthorEmail string `json:"author_email"`
	AuthorName  string `json:"author_name"`
	AuthorID    string `json:"author_id"`
	PhotoURL    string `json:"photo_url"`
	CreatedDate string `json:"created_date"`
}

var test_global_feed = []FeedItem{
	{ID:"3", CreatedDate:"2016-10-19", AuthorID: uuid.New(), AuthorName:"thierry", AuthorEmail:"no_gravatar@example.com", PhotoURL:"http://i1054.photobucket.com/albums/s499/vadimzbanok/1327.jpg"},
	{ID:"1", CreatedDate:"2016-10-17", AuthorID: uuid.New(), AuthorName:"ian", AuthorEmail:"ian.douglas@iandouglas.com", PhotoURL:"http://greenhackathon.com/graphics/greenhackathon_logo.png"},
	{ID:"2", CreatedDate:"2016-10-18", AuthorID: uuid.New(), AuthorName:"josh", AuthorEmail:"joshtilton@gmail.com", PhotoURL:"http://clubpenguin.wikia.com/wiki/File:Funny_RP.PNG"},
}
var test_user_feed = []FeedItem{
	{ID:"3", CreatedDate:"2016-10-19", AuthorID: uuid.New(), AuthorName:"thierry", AuthorEmail:"no_gravatar@example.com", PhotoURL:"https://pixabay.com/static/uploads/photo/2016/03/28/12/35/cat-1285634_1280.png"},
	{ID:"2", CreatedDate:"2016-10-18", AuthorID: uuid.New(), AuthorName:"josh", AuthorEmail:"joshtilton@gmail.com", PhotoURL:"https://pixabay.com/static/uploads/photo/2015/04/14/08/52/lion-721836_1280.jpg"},
}

var dbmap = initDb()

func initDb() *gorp.DbMap {
	db, err := sql.Open("mysql", "stream:B4ck3nd!@/stream_backend?parseTime=true")
	if err != nil || db == nil {
		panic("failed to connect database: " + err.Error())
	}
	dbmap := &gorp.DbMap{Db: db, Dialect: gorp.MySQLDialect{"InnoDB", "UTF8"}}
	dbmap.AddTableWithName(User{}, "users").SetKeys(true, "ID")
	dbmap.AddTableWithName(Photo{}, "photos").SetKeys(true, "ID")
	err = dbmap.CreateTablesIfNotExists()
	if err != nil {
		panic("failed to create tables: " + err.Error())
	}
	return dbmap
}

var StreamClient = initStream()
// Stream.io variables
var globalFeed *getstream.FlatFeed

func initStream() *getstream.Client {
	// GetStream.io setup
	client, err := getstream.New(&getstream.Config{
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
	return client
}

func main() {
	// S3
	//Endpoint:         "s3.amazonaws.com"
	//S3ForcePathStyle: true
	//S3Client = s3.New(session.New(&aws.Config{Region: aws.String("us-east-1")}))
	//_, err = S3Client.CreateBucket(&s3.CreateBucketInput{
	//	Bucket: &S3BucketName,
	//})
	//if err != nil {
	//	log.Println("Failed to create bucket", err)
	//	return
	//}
	//if err = S3Client.WaitUntilBucketExists(&s3.HeadBucketInput{Bucket: &S3BucketName}); err != nil {
	//	log.Printf("Failed to wait for bucket to exist %s, %s\n", S3BucketName, err)
	//	return
	//}

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

	router.GET("/users", getUsers)
	router.GET("/feed/:uuid", getFeed)
	router.GET("/follow/:target", getFollow)
	router.GET("/unfollow/:target", getUnfollow)
	//router.POST("/like/:uuid", postLikePhoto)
	//router.POST("/unlike/:uuid", postUnlikePhoto)

	router.GET("/testfeed", func(c *gin.Context) {
		// redirect to the repo, blog post, etc.
		//c.Redirect(http.StatusTemporaryRedirect, "//getstream.io/blog")
		c.JSON(http.StatusOK, gin.H{
			"feed": test_global_feed,
		})
	})
	router.POST("/login", postLogin)
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
	router.Run("0.0.0.0:3000")
}

func getFeed(c *gin.Context) {
	userUUID := c.Param("uuid")
	lastUUID := c.Query("lastUUID")

	log.Println("fetching feed for user", userUUID)
	// get user stream, send back in json format
	userFeed, err := StreamClient.FlatFeed("user", userUUID)
	if err != nil {
		log.Println("fetch feed threw an error")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
	}
	var options getstream.GetFlatFeedInput
	options.Limit = 50
	if lastUUID != "" {
		options.IDGT = lastUUID
	}
	activities, err := userFeed.Activities(&options)

	log.Println("returning activities")

	c.JSON(http.StatusOK, gin.H{
		"uuid": userUUID,
		"feed": activities,
	})
}

func getFollow(c *gin.Context) {
	userUUID := c.Query("uuid")
	if userUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user UUID not found"})
		return
	}

	followUUID := c.Param("target")
	if followUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target feed UUID not found"})
		return
	}

	userFeed, err := StreamClient.FlatFeed("user", userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	targetFeed, err := StreamClient.FlatFeed("user", followUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	userFeed.FollowFeedWithCopyLimit(targetFeed, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
	})
}

func getUnfollow(c *gin.Context) {
	userUUID := c.Query("uuid")
	if userUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user UUID not found"})
		return
	}

	unfollowUUID := c.Param("target")
	if unfollowUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target feed UUID not found"})
		return
	}

	userFeed, err := StreamClient.FlatFeed("user", userUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	targetFeed, err := StreamClient.FlatFeed("user", unfollowUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	userFeed.Unfollow(targetFeed)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
	})
}

func postPhotoUpload(c *gin.Context) {
	// handle upload in the background
	userUUID := c.PostForm("uuid")
	var user_id int = 0
	err := dbmap.SelectOne(&user_id, "SELECT id FROM users WHERE uuid=?", userUUID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if user_id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid not valid, photo not processed"})
		return
	}

	_, header, err := c.Request.FormFile("upload")
	log.Println(header.Filename)
	localFilename := "./tmp/" + uuid.New() + ".png"
	localSavedFile, err := os.Create(localFilename)
	if err != nil {
		log.Fatal(err)
	}
	localSavedFile.Close()

	var photo Photo
	photo.UUID = uuid.New()
	photo.UserID = user_id

	insert, err := dbmap.Exec(`
		INSERT INTO photos (uuid, user_id, CreatedAt, UpdatedAt)
		VALUES (?, ?, ?, ?)`,
		photo.UUID, user_id, time.Now(), time.Now())
	if err != nil {
		log.Println("sending error after photo insert")
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}
	_, err = insert.LastInsertId()
	if err == nil {
		log.Println("sending user payload response")
		c.JSON(http.StatusCreated, gin.H{"uuid": photo.UUID, "status": "processing"})
	} else {
		log.Println("sending error response from insert")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	// create copy to be used inside the goroutine
	cCp := c.Copy()
	go func() {
		log.Println("doing upload etc in the background")
		var photoFilename string

		// shrink image
		inImage, err := imaging.Open(localFilename)
		if err != nil {
			panic(err)
		}
		dstImage := imaging.Resize(inImage, 1024, 768, imaging.NearestNeighbor)
		imaging.Save(dstImage, photoFilename)

		// push to S3, get URL
		log.Println("push to S3")
		file, err := os.Open(localFilename)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		fileInfo, _ := file.Stat()
		var size int64 = fileInfo.Size()
		buffer := make([]byte, size)
		file.Read(buffer)
		fileBytes := bytes.NewReader(buffer) // convert to io.ReadSeeker type
		fileType := http.DetectContentType(buffer)
		path := "photos/" + file.Name()
		params := &s3.PutObjectInput{
			Bucket:        aws.String(S3BucketName), // required
			Key:           aws.String(path), // required
			ACL:           aws.String("public-read"),
			Body:          fileBytes,
			ContentLength: &size,
			ContentType:   aws.String(fileType),
			Metadata: map[string]*string{
				"Key": aws.String("MetadataValue"), //required
			},
			// see more at http://godoc.org/github.com/aws/aws-sdk-go/service/s3#S3.PutObject
		}
		result, err := S3Client.PutObject(params)
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				fmt.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
				if reqErr, ok := err.(awserr.RequestFailure); ok {
					// A service error occurred
					fmt.Println(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
				}
			} else {
				// This case should never be hit, the SDK should always return an
				// error which satisfies the awserr.Error interface.
				fmt.Println(err.Error())
			}
		}
		fmt.Println(awsutil.StringValue(result))
		photo.URL = "http://unknown.image"

		// send image url, date, username to stream

		now := time.Now()
		userFeed, err := StreamClient.FlatFeed("user", userUUID)
		if err != nil {
			fmt.Println(err)
		} else {
			_, err = globalFeed.AddActivity(&getstream.Activity{
				Verb:      "photo",
				ForeignID: photo.UUID,
				TimeStamp: &now,
				To:        []getstream.Feed{globalFeed, userFeed},
				Object:    getstream.FeedID(fmt.Sprintf("photo:%s", photo.UUID)),
				Actor:     getstream.FeedID(fmt.Sprintf("user:%s", userUUID)),
				MetaData:  map[string]string{
					// add as many custom keys/values here as you like
					"photoUrl": photo.URL,
				},
			})
			if err != nil {
				fmt.Println(err)
			}
		}
		// note that you are using the copied context "cCp", IMPORTANT
		log.Println("Done! in path " + cCp.Request.URL.Path)
	}()
}

func postRegister(c *gin.Context) {
	var user User
	var output []string

	email := c.PostForm("email")
	username := c.PostForm("username")
	log.Println(email, username)

	if username == "" || email == "" {
		if username == "" {
			output = append(output, "Username cannot be blank")
		}
		if email == "" {
			output = append(output, "Email cannot be blank")
		}
	} else {
		var user_id int = 0
		err := dbmap.SelectOne(&user_id, "SELECT id FROM users WHERE username=? or email=?",
			strings.ToLower(username),
			strings.ToLower(email))
		if err != nil && err.Error() != "sql: no rows in result set" {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		if user_id > 0 {
			output = append(output, "Username or Email already used")
		}
	}

	if len(output) > 0 {
		log.Println("sending friendly errors")
		c.JSON(http.StatusBadRequest, gin.H{"errors": output})
		return
	}

	log.Println("saving new user in db")
	user.Username = username
	user.Email = email
	user.UUID = uuid.New()
	insert, err := dbmap.Exec(`
		INSERT INTO users (uuid, username, email, CreatedAt, UpdatedAt)
		VALUES (?, ?, ?, ?, ?)`,
		user.UUID, strings.ToLower(user.Username), strings.ToLower(user.Email), time.Now(), time.Now())
	if err != nil {
		log.Println("sending error after insert")
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	id, err := insert.LastInsertId()
	if err == nil {
		log.Println("sending user payload response")
		c.JSON(http.StatusCreated, gin.H{"uuid": user.UUID, "user_id": id})
	} else {
		log.Println("sending error response from insert")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
	log.Println("ended up here, no response to send back")
}

func postLogin(c *gin.Context) {
	var user User
	var output []string

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
		err := dbmap.SelectOne(&user, "SELECT users.* FROM users WHERE username=? and email=?",
			strings.ToLower(username),
			strings.ToLower(email))
		if err != nil && err.Error() != "sql: no rows in result set" {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		if user.Username == "" {
			output = append(output, "Username or Email not found, please register")
		}
	}

	if len(output) > 0 {
		log.Println("sending friendly errors")
		c.JSON(http.StatusBadRequest, gin.H{"errors": output})
		return
	}

	c.JSON(http.StatusOK, gin.H{"UUID": user.UUID, "email": user.Email, "username": user.Username})
	return
}

func getUsers(c *gin.Context) {
	var users []User

	_, err := dbmap.Select(&users, "SELECT * FROM users ORDER BY username")
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, users)
	return
}

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
	"io"
	"path"
	"github.com/aws/aws-sdk-go/aws/session"
	"errors"
)

type User struct {
	gorm.Model
	UUID     string `gorm:"column:uuid" json:"uuid"`
	Username string `gorm:"column:username" json:"username"`
	Email    string `gorm:"column:email" json:"email"`
}

type UserItem struct {
	UUID      string `json:"uuid"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	DoIFollow bool `json:"doifollow"`
}

type Photo struct {
	gorm.Model
	UserID int  `gorm:"column:user_id,index" json:"user_id"`
	UUID   string `gorm:"column:uuid" json:"uuid"`
	URL    string `gorm:"column:url" json:"url"`
	Likes  int `gorm:"column:likes" json:"likes"`
}

var router *gin.Engine
var S3Client *s3.S3
var S3BucketName string = "android-demo"

type FeedItem struct {
	ID          string `json:"id"`
	AuthorEmail string `json:"author_email"`
	AuthorName  string `json:"author_name"`
	AuthorID    string `json:"author_id"`
	PhotoURL    string `json:"photo_url"`
	PhotoUUID   string `json:"photo_uuid"`
	DoIFollow   bool `json:"doifollow"`
	Likes       int `json:"likes"`
	ILikeThis   bool `json:"ilikethis"`
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
var aggregatedFeed *getstream.AggregatedFeed
var notificationFeed *getstream.NotificationFeed

func initStream() *getstream.Client {
	// GetStream.io setup
	client, err := getstream.New(&getstream.Config{
		APIKey:      os.Getenv("STREAM_API_KEY"),
		APISecret:   os.Getenv("STREAM_API_SECRET"),
		AppID:       os.Getenv("STREAM_APP_ID"),
		Location:    os.Getenv("STREAM_REGION"),
	})
	if err != nil {
		panic("failed to connect to stream: " + err.Error())
	}
	globalFeed, err = client.FlatFeed("user", "global")
	if err != nil {
		panic("could not set global feed")
	}
	aggregatedFeed, err = client.AggregatedFeed("aggregated", "photos")
	if err != nil {
		panic("could not set aggregated feed")
	}
	notificationFeed, err = client.NotificationFeed("notification", "likes")
	if err != nil {
		panic("could not set notification feed")
	}
	return client
}

func main() {
	// S3
	//Endpoint:         "s3.amazonaws.com"
	//S3ForcePathStyle: true
	S3Client = s3.New(session.New(&aws.Config{Region: aws.String("us-east-1")}))

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
	router.GET("/photolikes", getPhotoLikes)
	router.GET("/mylikes", getMyLikes)
	router.GET("/feed/:uuid", getFeed)
	router.GET("/myfollows", getMyFollows)
	router.GET("/follow/:target", getFollow)
	router.GET("/unfollow/:target", getUnfollow)
	router.GET("/likephoto/:photoUUID", getLikePhoto)
	router.GET("/unlikephoto/:photoUUID", getUnlikePhoto)

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
	var me User
	var err error

	userUUID := c.Param("uuid")
	log.Println("userUUID:", userUUID)

	myUUID := c.Query("uuid")
	log.Println("myUUID:", myUUID)

	if userUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user UUID not found"})
		return
	}

	if userUUID != "global" {
		_, err = validateUser(userUUID)
		if err != nil {
			if err.Error() == "not found" {
				c.JSON(http.StatusNotFound, err.Error())
			} else {
				log.Println(err.Error())
				c.JSON(http.StatusInternalServerError, err.Error())
			}
			return
		}
	}

	if myUUID != "" {
		me, err = validateUser(myUUID)
		if err != nil {
			if err.Error() == "not found" {
				c.JSON(http.StatusNotFound, err.Error())
			} else {
				log.Println(err.Error())
				c.JSON(http.StatusInternalServerError, err.Error())
			}
			return
		}
	}

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
	feedActivities, err := userFeed.Activities(&options)
	var activities []FeedItem
	var newestActivityUUID string

	for idx, activity := range feedActivities.Activities {
		log.Println("------------------\nactivity ID:", activity.ID)
		log.Println("activity ForeignID:", activity.ForeignID)
		if idx == 0 {
			newestActivityUUID = activity.ID
		}
		var user User
		bits := strings.Split(string(activity.Actor), ":")
		actor := bits[1]
		err := dbmap.SelectOne(&user, "SELECT users.* FROM users WHERE uuid=?", actor)
		if err != nil {
			log.Println(err.Error())
			continue
		}

		var doIFollow bool = false;
		if me.ID > 0 {
			doIFollow, err = fetchDoIFollow(me.ID, user.ID)
			if err != nil {
				log.Println("fetchDoIFollow error:", err)
			}
		} else {
			log.Println("skipped follow check, me.ID:", me.ID)
		}

		if activity.MetaData["photoUrl"] == "http://unknown.image" {
			log.Println("skipping bad url")
			continue
		}

		photo, err := validatePhoto(activity.ForeignID)
		if err != nil {
		log.Println("fetchDoILikePhoto error:", err)
			continue
		}
		log.Println("url:", photo.URL)
		log.Println("ID:", photo.ID)
		count, _ := fetchPhotoLikes(photo.ID)
		var like bool = false
		if me.ID > 0 {
			like, err = fetchDoILikePhoto(me.ID, photo.ID)
			if err != nil {
				log.Println("fetchDoILikePhoto error:", err)
			}
		} else {
			log.Println("skipped photo like check, me.ID:", me.ID)
		}

		activities = append(activities, FeedItem{
			AuthorEmail: user.Email,
			AuthorID: user.UUID,
			AuthorName: user.Username,
			Likes: count,
			ILikeThis: like,
			DoIFollow: doIFollow,
			PhotoURL: activity.MetaData["photoUrl"],
			PhotoUUID: photo.UUID,
			ID: activity.ForeignID,
			CreatedDate: activity.TimeStamp.Format("2006-01-02T15:04:05.999999"),
		})
	}

	log.Println("returning activities")

	if len(activities) == 0 {
		activities = []FeedItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"uuid": userUUID,
		"newest_activity_id": newestActivityUUID,
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

	var user User
	var uid1 uint = 0
	var uid2 uint = 0
	err := dbmap.SelectOne(&user, "SELECT users.* FROM users WHERE uuid=?", userUUID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if user.ID > 0 {
		uid1 = user.ID
	}
	err = dbmap.SelectOne(&user, "SELECT users.* FROM users WHERE uuid=?", followUUID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if user.ID > 0 {
		uid2 = user.ID
	}
	if uid1 <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "your UUID was not valid"})
		return
	}
	if uid2 <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "follow UUID was not valid"})
		return
	}

	var follow_id uint = 0
	err = dbmap.SelectOne(&follow_id, "SELECT id FROM follows WHERE user_id_1=? AND user_id_2=?", uid1, uid2)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if follow_id <= 0 {

		_, err = dbmap.Exec(`INSERT INTO follows (user_id_1, user_id_2) VALUES (?, ?)`, uid1, uid2)
		if err != nil {
			log.Println("sending error after insert")
			c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
			return
		}

		userFeed, err := StreamClient.FlatFeed("user", userUUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		targetFeed, err := StreamClient.FlatFeed("user", followUUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		userFeed.FollowFeedWithCopyLimit(targetFeed, 50)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
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
		return
	}

	targetFeed, err := StreamClient.FlatFeed("user", unfollowUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userFeed.Unfollow(targetFeed)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var user User
	var uid1, uid2 uint
	err = dbmap.SelectOne(&user, "SELECT users.* FROM users WHERE uuid=?", userUUID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if user.ID > 0 {
		uid1 = user.ID
	}
	err = dbmap.SelectOne(&user, "SELECT users.* FROM users WHERE uuid=?", unfollowUUID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if user.ID > 0 {
		uid2 = user.ID
	}
	if uid1 > 0 && uid2 > 0 {
		dbmap.Exec("delete from follows where user_id_1=? and user_id_2=?", uid1, uid2)
	} else if uid1 <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "your UUID was not valid"})
		return
	} else if uid2 <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unfollow UUID was not valid"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
	})
}

func getLikePhoto(c *gin.Context) {
	userUUID := c.Query("uuid")
	photoUUID := c.Param("photoUUID")

	log.Println("user", userUUID, "photo", photoUUID)

	user, err := validateUser(userUUID)
	if err != nil {
		if err.Error() == "not found" {
			log.Println("getLikePhoto, user uuid not found")
			c.JSON(http.StatusNotFound, "user " + err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	photo, err := validatePhoto(photoUUID)
	if err != nil {
		if err.Error() == "not found" {
			log.Println("getLikePhoto, photo uuid not found")
			c.JSON(http.StatusNotFound, "photo " + err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	insert, err := dbmap.Exec(`INSERT INTO likes (user_id, photo_id) VALUES (?, ?)`, user.ID, photo.ID)
	if err != nil {
		log.Println("sending error after insert")
		c.JSON(http.StatusInternalServerError, gin.H{"errors": err.Error()})
		return
	}
	foreign_id, err := insert.LastInsertId()
	if err != nil {
		log.Println("sending error response from insert")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	// TODO update feeds

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"foreign_id": foreign_id,
	})
}

func getUnlikePhoto(c *gin.Context) {
	userUUID := c.Query("uuid")
	photoUUID := c.Param("photoUUID")

	log.Println("user", userUUID, "photo", photoUUID)

	user, err := validateUser(userUUID)
	if err != nil {
		if err.Error() == "not found" {
			log.Println("getLikePhoto, user uuid not found")
			c.JSON(http.StatusNotFound, "user " + err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	photo, err := validatePhoto(photoUUID)
	if err != nil {
		if err.Error() == "not found" {
			log.Println("getLikePhoto, photo uuid not found")
			c.JSON(http.StatusNotFound, "photo " + err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	log.Println(user.ID, photo.ID)

	var foreign_id int = 0
	err = dbmap.SelectOne(&foreign_id, `SELECT id FROM likes WHERE user_id=? AND photo_id=?`, user.ID, photo.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	_, err = dbmap.Exec("DELETE FROM likes WHERE id=?", foreign_id)
	if err != nil {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	// TODO alter feeds

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"foreign_id": foreign_id,
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

	file, header, err := c.Request.FormFile("upload")
	log.Println(header.Filename)
	localFilename := "./tmp/" + uuid.New() + ".png"
	localSavedFile, err := os.Create(localFilename)
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.Copy(localSavedFile, file)
	if err != nil {
		log.Fatal(err)
	}
	localSavedFile.Close()

	var photo Photo
	photo.UUID = uuid.New()
	photo.UserID = user_id

	insert, err := dbmap.Exec(`
		INSERT INTO photos (UUID, UserID, CreatedAt, UpdatedAt, Likes)
		VALUES (?, ?, ?, ?, 0)`,
		photo.UUID, user_id, time.Now(), time.Now())
	if err != nil {
		log.Println("sending error after photo insert")
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}
	photo_id, err := insert.LastInsertId()
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
		log.Println("local filename:", localFilename)
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
		log.Println("file size", size)
		buffer := make([]byte, size)
		file.Read(buffer)
		fileBytes := bytes.NewReader(buffer) // convert to io.ReadSeeker type
		fileType := http.DetectContentType(buffer)
		path := "photos/" + path.Base(file.Name())
		log.Println("s3.PutObjectInput")
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
		log.Println(params)
		log.Println("s3.PutObject")
		result, err := S3Client.PutObject(params)
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				fmt.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
				if reqErr, ok := err.(awserr.RequestFailure); ok {
					// A service error occurred
					fmt.Println(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
					log.Println(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
				}
			} else {
				// This case should never be hit, the SDK should always return an
				// error which satisfies the awserr.Error interface.
				log.Println("s3.PutObject err:", err.Error())
				fmt.Println(err.Error())
			}
		}
		log.Println("s3.PutObject finished")
		fmt.Println(awsutil.StringValue(result))
		// we need to get the S3 URL from that result somehow
		photo.URL = "https://android-demo.s3.amazonaws.com/" + path

		_, err = dbmap.Exec(`
		UPDATE photos SET URL=?, UpdatedAt=? WHERE ID=?`,
			photo.URL, time.Now(), photo_id)
		if err != nil {
			log.Println("sending error after photo insert")
			c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
			return
		}

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
		var user User
		log.Println("username:", username)
		log.Println("email:", email)
		err := dbmap.SelectOne(&user, "SELECT * FROM users WHERE username=? AND email=?",
			strings.ToLower(username),
			strings.ToLower(email))
		if err != nil && err.Error() != "sql: no rows in result set" {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		if user.ID > 0 {
			// TODO we're cheating here, if the username/email is already registered, we'll just log them in
			log.Println("registration cheat! :)")
			c.JSON(http.StatusOK, gin.H{"uuid": user.UUID, "id": user.ID})
			return
		}
	}
	var id int64
	err := dbmap.SelectOne(&id, "SELECT id FROM users WHERE username=?", strings.ToLower(username))
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if id > 0 {
		output = append(output, "username already in use")
	}
	err = dbmap.SelectOne(&id, "SELECT id FROM users WHERE email=?", strings.ToLower(email))
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if id > 0 {
		output = append(output, "email already in use")
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

	id, err = insert.LastInsertId()
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

/*
  http://localhost:3000/users
  returns array of user objects
	{"users":[{"ID":1,"uuid":"9cf34d34-a042-4231-babc-eee6ba67bd18","username":"ian","email":"ian@example.com"},{...}, ...]}
 */
func getUsers(c *gin.Context) {
	var data []User
	var users []UserItem

	userUUID := c.Query("uuid")
	if userUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user UUID not found"})
		return
	}
	log.Println("who's asking for the user list:", userUUID)
	user, err := validateUser(userUUID)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "user UUID" + err.Error()})
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	_, err = dbmap.Select(&data, "SELECT * FROM users ORDER BY username")
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	for _, oneUser := range data {
		doIFollow, _ := fetchDoIFollow(user.ID, oneUser.ID)
		userItem := UserItem{
			UUID: oneUser.UUID,
			Username: oneUser.Username,
			Email: oneUser.Email,
			DoIFollow: doIFollow,
		}
		users = append(users, userItem)
	}
	c.JSON(http.StatusOK, gin.H{"users": users})

	return
}

/*
  http://localhost:3000/photolikes?uuid=3c7c77bd-e1b4-4e64-9c9d-fff223efc17b
  returns count of likes for a photo's UUID
	{"likes":23}
 */
func getPhotoLikes(c *gin.Context) {
	var photo Photo;
	var count int = 0;

	photoUUID := c.Query("uuid")
	photo, err := validatePhoto(photoUUID)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	count, err = fetchPhotoLikes(photo.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"likes": count})
	return
}

/*
  http://localhost:3000/mylikes?uuid=9cf34d34-a042-4231-babc-eee6ba67bd18
  returns list of photo UUIDs you liked:
	{"photos_liked":["3c7c77bd-e1b4-4e64-9c9d-fff223efc17b", "...", ...]}
 */
func getMyLikes(c *gin.Context) {
	var user User;

	userUUID := c.Query("uuid")
	user, err := validateUser(userUUID)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "user UUID" + err.Error()})
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	photo_likes := []string{}

	_, err = dbmap.Select(&photo_likes, `
		SELECT p.UUID
		FROM photos p
		  JOIN likes on p.ID=likes.photo_id
		WHERE likes.user_id=?`, user.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"photos_liked": photo_likes})

	return
}

/*
  http://localhost:3000/myfollows?uuid=9cf34d34-a042-4231-babc-eee6ba67bd18
  returns list of user UUIDs you follow:
	{"users_followed":["03a1cfed-3590-4aa8-a592-f78bc71ccfbd", "...", ...]}
 */
func getMyFollows(c *gin.Context) {
	var user User;

	userUUID := c.Query("uuid")
	user, err := validateUser(userUUID)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	follows := []string{}

	_, err = dbmap.Select(&follows, `
		SELECT u.UUID
		FROM users u
		  JOIN follows f ON f.user_id_2=u.ID
		WHERE f.user_id_1=?`, user.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"users_followed": follows})

	return
}

/* helper functions */

func fetchDoILikePhoto(myID uint, photoID uint) (bool, error) {
	var rowID int = 0
	err := dbmap.SelectOne(&rowID, `
		SELECT id
		FROM likes
		WHERE user_id=? AND photo_id=?`, myID, photoID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return false, err
	}
	if rowID > 0 {
		return true, nil
	}
	return false, nil
}

func fetchDoIFollow(myID uint, userID uint) (bool, error) {
	var rowID int = 0
	log.Println("fetchDoIFollow ids:", myID, userID)

	err := dbmap.SelectOne(&rowID, `
		SELECT id
		FROM follows
		WHERE user_id_1=? AND user_id_2=?`, myID, userID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println("fetchDoIFollow 1")
		return false, err
	}
	if rowID > 0 {
		log.Println("fetchDoIFollow 2")
		return true, nil
	}
	log.Println("fetchDoIFollow 3")
	return false, nil
}

func fetchPhotoLikes(photoID uint) (int, error) {
	var count int = 0
	err := dbmap.SelectOne(&count, "SELECT count(*) FROM likes WHERE photo_id=?", photoID)
	if err != nil {
		return -1, err
	}
	return count, nil

}

func validateUser(userUUID string) (User, error) {
	//return validateRow(userUUID, "users", "User")
	var data User
	err := dbmap.SelectOne(&data, "SELECT * FROM users WHERE UUID=?", userUUID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("not found")
		}
		return data, err
	}
	return data, nil
}

func validatePhoto(photoUUID string) (Photo, error) {
	//return validateRow(photoUUID, "photos", "Photo")
	var data Photo
	err := dbmap.SelectOne(&data, "SELECT * FROM photos WHERE UUID=?", photoUUID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("not found")
		}
		log.Println("validate photo url", data.URL)
		log.Println("validate photo err", err)
		return data, err
	}
	log.Println("photo validate, returning payload")
	return data, nil

}

//func validateRow(strUUID string, table string, kind string) (interface{}, error) {
//	var whatKind *interface{}
//	if strUUID == "" {
//		return nil, errors.New("not found")
//	}
//	if kind == "User" {
//		whatKind = &User{}
//	}
//	if kind == "Photo" {
//		whatKind = &Photo{}
//	}
//
//	err := dbmap.SelectOne(whatKind, "SELECT * FROM " + table + " WHERE UUID=?", strUUID)
//	if err != nil {
//		if err.Error() == "sql: no rows in result set" {
//			err = errors.New("not found")
//		}
//		return nil, err
//	}
//	return kind, nil
//}
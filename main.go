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
	//"github.com/aws/aws-sdk-go/aws/awsutil"
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
	UUID     string `json:"uuid"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type Likes struct {
	gorm.Model
	UserID  uint
	PhotoID uint
	UUID    string `json:"uuid"`
}

type UserItem struct {
	UUID      string `json:"uuid"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	DoIFollow bool `json:"doifollow"`
}

type Photo struct {
	gorm.Model
	UserID uint  `json:"user_id"`
	UUID   string `json:"uuid"`
	URL    string `json:"url"`
	Likes  int `json:"likes"`
}
type Follows struct {
	gorm.Model
	UserID1 uint  `json:"user_id_1"`
	UserID2 uint  `json:"user_id_2"`
	UUID    string `json:"uuid"`
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
	CreatedDate int64 `json:"created_date"`
}
type AggregatedFeedItem struct {
	AuthorEmail string `json:"author_email"`
	AuthorName  string `json:"author_name"`
	AuthorID    string `json:"author_id"`
	Photos      []string `json:"photos"`
	CreatedDate int64 `json:"created_date"`
}

type NotificationActor struct {
	AuthorEmail string `json:"author_email"`
	AuthorName  string `json:"author_name"`
}

type NotificationLike struct {
	PhotoURL string `json:"photo_url"`
	Actors   []NotificationActor `json:"actors"`
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
	dbmap.AddTableWithName(Likes{}, "likes").SetKeys(true, "ID")
	dbmap.AddTableWithName(Follows{}, "follows").SetKeys(true, "ID")
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
	// set up S3
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

	router.POST("/register", postRegister) // also does login

	router.GET("/users", getUsers)
	router.GET("/follow/:targetUUID", getFollow)
	router.GET("/unfollow/:targetUUID", getUnfollow)
	router.GET("/likephoto/:photoUUID", getLikePhoto)
	router.GET("/unlikephoto/:photoUUID", getUnlikePhoto)
	router.GET("/profilestats/:myUUID", getUserProfileStats)

	/* get user feeds

	Best Practices:
	- send myUUID on calls so you can determine on your return payload whether your user follows the author
	- send &newestActivityUUID= for pulling newer items later so you don't refetch the entire feed every time
	  - this code will send "newest_activity_id" to assist with this

	// get global feed, myUUID is optional
	http://localhost:3000/feed/user/global?myUUID=
	http://localhost:3000/feed/user/global?myUUID=9cf34d34-a042-4231-babc-eee6ba67bd18

	// get one user's feed data (to see your own feed of items)
	// sending myUUID for a different user will show whether you follow them or liked their photos
	http://localhost:3000/feed/user/9cf34d34-a042-4231-babc-eee6ba67bd18?myUUID=9cf34d34-a042-4231-babc-eee6ba67bd18
	*/
	router.GET("/feed/notifications", func(c *gin.Context) {
		var statusCode int
		var payload gin.H

		feedSlug := "notification"
		myUserUUID := c.Query("myUUID")

		if myUserUUID == "" {
			statusCode = http.StatusNotFound
			payload = gin.H{"error": "missing myUUID parameter"}
		} else {
			lastActivityUUID := c.Query("newestActivityUUID")
			statusCode, payload = getNotificationFeed(feedSlug, myUserUUID, lastActivityUUID);
		}
		c.JSON(statusCode, payload)
	})
	router.GET("/feed/user/:feedUUID", func(c *gin.Context) {
		var statusCode int
		var payload gin.H

		feedStub := "user"
		feedUUID := c.Param("feedUUID")
		myUserUUID := c.Query("myUUID")

		lastActivityUUID := c.Query("newestActivityUUID")
		statusCode, payload = getFlatFeed(feedStub, feedUUID, myUserUUID, lastActivityUUID);
		c.JSON(statusCode, payload)
	})
	router.GET("/feed/timeline/:feedUUID", func(c *gin.Context) {
		var statusCode int
		var payload gin.H

		feedStub := "timeline"
		feedUUID := c.Param("feedUUID")
		myUserUUID := c.Query("myUUID")

		if feedUUID == "global" {
			statusCode = http.StatusNotFound
			payload = gin.H{"error": "global timeline feed does not exist"}
		} else {
			lastActivityUUID := c.Query("newestActivityUUID")
			statusCode, payload = getFlatFeed(feedStub, feedUUID, myUserUUID, lastActivityUUID);
		}
		c.JSON(statusCode, payload)
	})
	router.GET("/feed/timeline_aggregated/:feedUUID", func(c *gin.Context) {
		var statusCode int
		var payload gin.H

		feedSlug := "timeline_aggregated"
		feedUUID := c.Param("feedUUID")
		myUserUUID := c.Query("myUUID")

		if feedUUID == "global" {
			statusCode = http.StatusNotFound
			payload = gin.H{"error": "global timeline aggregated feed does not exist"}
		} else {
			lastActivityUUID := c.Query("newestActivityUUID")
			statusCode, payload = getAggregatedFeed(feedSlug, feedUUID, myUserUUID, lastActivityUUID);
		}
		c.JSON(statusCode, payload)
	})


	// post a photo to global feed and user's feed
	router.POST("/upload", postPhotoUpload)

	// no more custom code under here
	//router.Static("/", "index.html")
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
		})
	})
	log.Print("Listening on port 3000")
	router.Run("0.0.0.0:3000")
}

func getFlatFeed(
feedSlug string,
feedUserUUID string,
myUserUUID string,
lastActivityUUID string,
) (int, map[string]interface{}) {
	var err error
	var activities []FeedItem
	var newestActivityUUID string
	var me User

	if feedUserUUID == "" {
		return http.StatusBadRequest, gin.H{"error": "user UUID not found"}
	}
	if feedUserUUID != "global" {
		_, err := validateUser(feedUserUUID)
		if err != nil {
			if err.Error() == "not found" {
				return http.StatusNotFound, gin.H{"error": "user " + err.Error()}
			} else {
				log.Println(err.Error())
				return http.StatusInternalServerError, gin.H{"error": err.Error()}
			}
		}
	}
	if myUserUUID != "" {
		me, err = validateUser(myUserUUID)
		if err != nil {
			log.Println("validate user threw an error:", err)
			if err.Error() == "not found" {
				return http.StatusNotFound, gin.H{"error": err.Error()}
			} else {
				return http.StatusInternalServerError, gin.H{"error": err.Error()}
			}
		}
	}

	var options getstream.GetFlatFeedInput
	options.Limit = 100
	if lastActivityUUID != "" {
		options.IDGT = lastActivityUUID
	}
	streamFeed, err := StreamClient.FlatFeed(feedSlug, feedUserUUID)
	if err != nil {
		log.Println("fetch feed threw an error")
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}

	feedActivities, err := streamFeed.Activities(&options)

	activities = parseFlatFeed(me, feedSlug, feedActivities.Activities)
	newestActivityUUID = ""
	if len(activities) > 0 {
		newestActivityUUID = activities[0].ID
	}

	if len(activities) == 0 {
		activities = []FeedItem{}
	}

	return http.StatusOK, gin.H{
		"uuid": feedUserUUID,
		"newest_activity_id": newestActivityUUID,
		"feed": activities,
	}
}
func parseFlatFeed(me User, feedSlug string, inActivities []*getstream.Activity) []FeedItem {
	var activities []FeedItem
	var doIFollowUser bool = false
	var doILikePhoto bool = false

	for _, activity := range inActivities {
		bits := strings.Split(string(activity.Actor), ":")
		actorUUID := bits[1]
		user, err := validateUser(actorUUID)
		if err != nil {
			log.Println("skipping activity, validating activity actor failed:", err.Error())
			continue
		}

		if me.ID > 0 {
			doIFollowUser, err = fetchDoIFollow(me.ID, user.ID)
			if err != nil {
				log.Println("fetchDoIFollow error:", err)
				// TODO deal with database error?
			}
		} else if feedSlug == "timeline" {
			// you'd only be seeing this in your timeline if you're following them, so we'll force true
			doIFollowUser = true;
		}

		photo, err := validatePhoto(activity.ForeignID)
		if err != nil {
			log.Println("validatePhoto error:", err)
			continue
		}

		count, _ := fetchPhotoLikes(photo.ID)
		if me.ID > 0 {
			doILikePhoto, err = fetchDoILikePhoto(me.ID, photo.ID)
			if err != nil {
				log.Println("fetchDoILikePhoto error:", err)
			}
		}

		activities = append(activities, FeedItem{
			AuthorEmail: user.Email,
			AuthorID: user.UUID,
			AuthorName: user.Username,
			Likes: count,
			ILikeThis: doILikePhoto,
			DoIFollow: doIFollowUser,
			PhotoURL: activity.MetaData["photoUrl"],
			PhotoUUID: photo.UUID,
			ID: activity.ForeignID,
			CreatedDate: activity.TimeStamp.Unix(),
		})
		log.Println(activities)
	}

	return activities
}

func getAggregatedFeed(
feedSlug string,
feedUserUUID string,
myUserUUID string,
lastActivityUUID string,
) (int, map[string]interface{}) {
	var err error
	var newestActivityUUID string

	if feedUserUUID == "" {
		return http.StatusBadRequest, gin.H{"error": "user UUID not found"}
	}
	if feedUserUUID != "global" {
		_, err := validateUser(feedUserUUID)
		if err != nil {
			if err.Error() == "not found" {
				return http.StatusNotFound, gin.H{"error": "user " + err.Error()}
			} else {
				log.Println(err.Error())
				return http.StatusInternalServerError, gin.H{"error": err.Error()}
			}
		}
	}
	if myUserUUID != "" {
		_, err = validateUser(myUserUUID)
		if err != nil {
			log.Println("validate user threw an error:", err)
			if err.Error() == "not found" {
				return http.StatusNotFound, gin.H{"error": err.Error()}
			} else {
				log.Println(err.Error())
				return http.StatusInternalServerError, gin.H{"error": err.Error()}
			}
		}
	}

	var options getstream.GetAggregatedFeedInput
	options.Limit = 100
	if lastActivityUUID != "" {
		options.IDGT = lastActivityUUID
	}
	streamFeed, err := StreamClient.AggregatedFeed(feedSlug, feedUserUUID)
	if err != nil {
		log.Println("fetch feed threw an error")
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}

	feedActivities, err := streamFeed.Activities(&options)
	activities := parseAggregatedFeed(feedActivities)

	newestActivityUUID = feedActivities.Next

	return http.StatusOK, gin.H{
		"uuid": feedUserUUID,
		"newest_activity_id": newestActivityUUID,
		"feed": activities,
	}
}
func parseAggregatedFeed(inActivities *getstream.GetAggregatedFeedOutput) []AggregatedFeedItem {
	activities := []AggregatedFeedItem{}

	for _, result := range inActivities.Results {

		groupBits := strings.Split(result.Group, "_")
		userBits := strings.Split(groupBits[0], ":")
		actorUUID := userBits[1]
		actor, err := validateUser(actorUUID)
		if err != nil {
			log.Println("actvity actor validateUser error:", err)
			continue
		}

		value := result.CreatedAt
		layout := "2006-01-02T15:04:05.999999"
		t, err := time.Parse(layout, value)
		if err != nil {
			log.Println(err)
		}

		aggActivity := AggregatedFeedItem{
			CreatedDate: t.Unix(),
			AuthorEmail: actor.Email,
			AuthorName: actor.Username,
			AuthorID: actor.UUID,
		}
		photos := []string{}
		for _, activity := range result.Activities {

			p, err := validatePhoto(activity.ForeignID)
			if err != nil {
				log.Println("validatePhoto error:", err)
				continue
			}

			photos = append(photos, p.URL)
		}
		aggActivity.Photos = photos
		activities = append(activities, aggActivity)
	}

	return activities
}

func getNotificationFeed(
feedSlug string,
myUserUUID string,
lastActivityUUID string,
) (int, map[string]interface{}) {
	var err error
	var newestActivityUUID string

	_, err = validateUser(myUserUUID)
	if err != nil {
		if err.Error() == "not found" {
			return http.StatusNotFound, gin.H{"error": "user " + err.Error()}
		} else {
			log.Println(err.Error())
			return http.StatusInternalServerError, gin.H{"error": err.Error()}
		}
	}

	var options getstream.GetNotificationFeedInput
	options.Limit = 100
	if lastActivityUUID != "" {
		options.IDGT = lastActivityUUID
	}
	streamFeed, err := StreamClient.NotificationFeed(feedSlug, myUserUUID)
	if err != nil {
		log.Println("fetch feed threw an error")
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}

	feedActivities, err := streamFeed.Activities(&options)
	activities := parseNotificationFeed(feedActivities)

	//newestActivityUUID = feedActivities.Next

	return http.StatusOK, gin.H{
		"newest_activity_id": newestActivityUUID,
		"feed": activities,
	}
}
func parseNotificationFeed(inActivities *getstream.GetNotificationFeedOutput) []interface{} {
	track := []interface{}{}
	likes := make(map[string][]NotificationActor)
	follows := []NotificationActor{}

	for _, r := range inActivities.Results {
		verb := r.Verb
		if verb == "like" {
			for _, activity := range r.Activities {
				// who did this verb?
				bits := strings.Split(string(activity.Actor), ":")
				actor, _ := validateUser(bits[1])
				if actor.ID <= 0 {
					continue
				}

				photoUrl := activity.MetaData["photoUrl"]
				if _, ok := likes[photoUrl]; !ok {
					likes[photoUrl] = []NotificationActor{}
				}
				likes[photoUrl] = append(likes[photoUrl], NotificationActor{
					AuthorEmail: actor.Email,
					AuthorName: actor.Username,
				})
			}
		} else if verb == "follow" {
			for _, activity := range r.Activities {
				// who did this verb?
				bits := strings.Split(string(activity.Actor), ":")
				actor, _ := validateUser(bits[1])
				if actor.ID <= 0 {
					continue
				}
				follows = append(follows, NotificationActor{
					AuthorEmail: actor.Email,
					AuthorName: actor.Username,
				})
			}
		}
	}

	keys := make([]string, len(likes))

	i := 0
	for k := range likes {
		keys[i] = k
		i++
	}
	for _, photo_url := range keys {
		payload := NotificationLike{
			PhotoURL: photo_url,
			Actors: likes[photo_url],
		}
		track = append(track, map[string]interface{}{"verb": "like", "payload": payload})
	}

	tmpFollows := map[string]interface{}{}
	if len(follows) > 0 {
		tmpFollows = map[string]interface{}{"verb": "follow", "payload": follows}
		track = append(track, tmpFollows)
	}

	return track
}

/* best practice:
   my 'timeline' feed follows someone else's 'user' feed
 */
func getFollow(c *gin.Context) {
	var follow_id uint = 0

	myUUID := c.Query("myUUID")
	me, err := validateUser(myUUID)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "myUUID " + err.Error()})
			return
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	targetUUID := c.Param("targetUUID")
	if targetUUID == myUUID {
		c.JSON(http.StatusBadRequest, gin.H{"best_practice_violation": "users should not follow themselves"})
		return
	}
	target, err := validateUser(targetUUID)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "targetUUID " + err.Error()})
			return
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	err = dbmap.SelectOne(&follow_id, "SELECT id FROM follows WHERE UserID1=? AND UserID2=?", me.ID, target.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	var followUUID string = uuid.New()
	_, err = dbmap.Exec(`INSERT INTO follows (UserID1, UserID2, UUID) VALUES (?, ?, ?)`, me.ID, target.ID, followUUID)
	if err != nil {
		log.Println("sending error after insert")
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	myFeed, err := StreamClient.FlatFeed("timeline", myUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	myAggFeed, err := StreamClient.FlatFeed("timeline_aggregated", myUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	targetFeed, err := StreamClient.FlatFeed("user", targetUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// best practice
	// my timeline feed follows your user feed, so my timeline shows each of your individual events
	// my aggregated timeline feed follows your feed so my aggregated timeline shows "josh added two photos" etc
	myFeed.FollowFeedWithCopyLimit(targetFeed, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	myAggFeed.FollowFeedWithCopyLimit(targetFeed, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	targetNotFeed, err := StreamClient.NotificationFeed("notification", targetUUID)
	if err != nil {
		log.Println("couldn't connect to notification feed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	now := time.Now()
	_, err = targetNotFeed.AddActivity(&getstream.Activity{
		Verb:      "follow",
		ForeignID: followUUID,
		TimeStamp: &now,
		Object:    getstream.FeedID(fmt.Sprintf("user:%s", targetUUID)),
		Actor:     getstream.FeedID(fmt.Sprintf("user:%s", myUUID)),
	})
	if err != nil {
		log.Println("couldn't add follow activity to notification feed", err)
		c.JSON(http.StatusInternalServerError, gin.H{"errors": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

/* best practice:
   my 'timeline' feed unfollows someone else's 'user' feed
 */
func getUnfollow(c *gin.Context) {
	myUUID := c.Query("myUUID")
	me, err := validateUser(myUUID)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "myUUID " + err.Error()})
			return
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	targetUUID := c.Param("targetUUID")
	if targetUUID == myUUID {
		c.JSON(http.StatusBadRequest, gin.H{"best_practice_violation": "users should not unfollow themselves"})
		return
	}
	target, err := validateUser(targetUUID)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "targetUUID " + err.Error()})
			return
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	myFeed, err := StreamClient.FlatFeed("timeline", myUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	myAggFeed, err := StreamClient.FlatFeed("timeline_aggregated", myUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	targetFeed, err := StreamClient.FlatFeed("user", targetUUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// for unfollowing, you can also use UnfollowKeepingHistory if you want to keep old
	// activities in myUUID's timeline but nothing new as of right now, just change the
	// method name on the next line from .Unfollow() to .UnfollowKeepingHistory()
	err = myFeed.Unfollow(targetFeed)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	err = myAggFeed.Unfollow(targetFeed)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	targetNotFeed, err := StreamClient.NotificationFeed("notification", targetUUID)
	if err != nil {
		log.Println("couldn't connect to notification feed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var foreign_uuid string
	err = dbmap.SelectOne(&foreign_uuid, `SELECT uuid FROM follows WHERE UserID1=? AND UserID2=?`, me.ID, target.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println("select uuid from follows", err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	} else {
		err = targetNotFeed.RemoveActivityByForeignID(&getstream.Activity{ForeignID: foreign_uuid})
		if err != nil {
			log.Println("removing activity from stream failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"errors": err.Error()})
			return
		}
	}
	dbmap.Exec("DELETE FROM follows WHERE UserID1=? AND UserID2=?", me.ID, target.ID)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func getLikePhoto(c *gin.Context) {
	var targetUUID string

	myUUID := c.Query("myUUID")
	photoUUID := c.Param("photoUUID")

	user, err := validateUser(myUUID)
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

	var like Likes
	dbmap.SelectOne(&like, `SELECT ID,UUID,UserID,PhotoID FROM likes WHERE UserID=? AND PhotoID=? LIMIT 1`, user.ID, photo.ID)
	if like.ID > 0 {
		c.JSON(http.StatusOK, gin.H{"status": "you already like this"})
		return
	}

	err = dbmap.SelectOne(&targetUUID, "SELECT uuid FROM users WHERE id=?", photo.UserID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println("finding photo author failed:", err)
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	targetFeed, err := StreamClient.NotificationFeed("notification", targetUUID)
	if err != nil {
		log.Println("couldn't connect to notification feed:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	var likeUUID string = uuid.New()
	likeDBPayload, err := dbmap.Exec(`INSERT INTO likes (UserID, PhotoID, UUID, CreatedAt) VALUES (?, ?, ?, ?)`,
		user.ID, photo.ID, likeUUID, now)
	if err != nil {
		log.Println("sending error after insert")
		c.JSON(http.StatusInternalServerError, gin.H{"errors": err.Error()})
		return
	}
	likeID, err := likeDBPayload.LastInsertId()
	if err != nil {
		log.Println("sending error response from insert")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	_, err = targetFeed.AddActivity(&getstream.Activity{
		Verb:      "like",
		ForeignID: likeUUID,
		TimeStamp: &now,
		Object:    getstream.FeedID(fmt.Sprintf("photo:%s", photo.UUID)),
		Actor:     getstream.FeedID(fmt.Sprintf("user:%s", myUUID)),
		MetaData:  map[string]string{
			"photoUrl": photo.URL,
		},
	})
	if err != nil {
		log.Println("couldn't add activity to notification feed", err)
		c.JSON(http.StatusInternalServerError, gin.H{"errors": err.Error()})

		dbmap.Exec("DELETE FROM likes WHERE ID=?", likeID)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func getUnlikePhoto(c *gin.Context) {
	var targetUUID string

	myUUID := c.Query("myUUID")
	photoUUID := c.Param("photoUUID")

	user, err := validateUser(myUUID)
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

	err = dbmap.SelectOne(&targetUUID, "SELECT uuid FROM users WHERE id=?", photo.UserID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println("select uuid for photo author", err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	targetFeed, err := StreamClient.NotificationFeed("notification", targetUUID)
	if err != nil {
		log.Println("couldn't connect to notification feed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var like Likes
	err = dbmap.SelectOne(&like, `SELECT ID,UUID FROM likes WHERE UserID=? AND PhotoID=? LIMIT 1`, user.ID, photo.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println("select * from likes", err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	} else {
		err = targetFeed.RemoveActivityByForeignID(&getstream.Activity{ID: like.UUID})
		if err != nil && err.Error() != "no ForeignID" {
			log.Println("removing activity from stream failed:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"errors": err.Error()})
			return
		}

		_, err = dbmap.Exec("DELETE FROM likes WHERE ID=?", like.ID)
		if err != nil {
			log.Println("delete from likes, like ID", like.ID, err)
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
		return
	}
	c.JSON(http.StatusConflict, gin.H{"status": "no db entry for original like"})
}

func postPhotoUpload(c *gin.Context) {
	var me User

	myUUID := c.PostForm("myUUID")
	me, err := validateUser(myUUID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println("594", err.Error())
		c.JSON(http.StatusInternalServerError, "user " + err.Error())
		return
	}
	if me.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user uuid not valid, photo not processed"})
		return
	}

	file, _, err := c.Request.FormFile("upload")
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
	photo.UserID = me.ID

	insert, err := dbmap.Exec(`
		INSERT INTO photos (UUID, UserID, CreatedAt, UpdatedAt, Likes)
		VALUES (?, ?, ?, ?, 0)`,
		photo.UUID, me.ID, time.Now(), time.Now())
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

	go func() {
		//var photoFilename string

		// shrink image
		inImage, err := imaging.Open(localFilename)
		if err != nil {
			panic(err)
		}
		dstImage := imaging.Fit(inImage, 1024, 768, imaging.NearestNeighbor)
		imaging.Save(dstImage, localFilename)

		// push to S3, get URL
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
		path := "photos/" + path.Base(file.Name())
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
		_, err = S3Client.PutObject(params)
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok {
				log.Println(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
				if reqErr, ok := err.(awserr.RequestFailure); ok {
					// A service error occurred
					log.Println(reqErr.Code(), reqErr.Message(), reqErr.StatusCode(), reqErr.RequestID())
				}
			} else {
				// This case should never be hit, the SDK should always return an
				// error which satisfies the awserr.Error interface.
				log.Println("s3.PutObject err:", err.Error())
			}
		}

		now := time.Now()
		photo.URL = "https://android-demo.s3.amazonaws.com/" + path
		_, err = dbmap.Exec(`
			UPDATE photos SET URL=?, UpdatedAt=? WHERE ID=?`,
			photo.URL, now, photo_id)
		if err != nil {
			log.Println("sending error after photo insert")
			c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
			return
		}

		globalFeed, err := StreamClient.FlatFeed("user", "global")
		if err != nil {
			log.Println(err)
		}
		userFeed, err := StreamClient.FlatFeed("user", myUUID)
		if err != nil {
			log.Println(err)
		} else {
			_, err = globalFeed.AddActivity(&getstream.Activity{
				Verb:      "photo",
				ForeignID: photo.UUID,
				TimeStamp: &now,
				To:        []getstream.Feed{userFeed},
				Object:    getstream.FeedID(fmt.Sprintf("photo:%s", photo.UUID)),
				Actor:     getstream.FeedID(fmt.Sprintf("user:%s", myUUID)),
				MetaData:  map[string]string{
					// add as many custom keys/values here as you like
					"photoUrl": photo.URL,
				},
			})
			if err != nil {
				fmt.Println(err)
			}
		}
	}()
}

/* we took a shortcut on authentication where a user 'registering' with the same username/email
   already in the database would log in that user. This, of course, is not authentication best
   practice, but a proper auth workflow is outside the scope of this project.
 */
func postRegister(c *gin.Context) {
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
		var user User
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
}

/*
  http://localhost:3000/users?myUUID=9cf34d34-a042-4231-babc-eee6ba67bd18
  returns array of user objects
	{"users":[{"ID":1,"uuid":"9cf34d34-a042-4231-babc-eee6ba67bd18","username":"ian","email":"ian@example.com"},{...}, ...]}
 */
func getUsers(c *gin.Context) {
	var data []User
	var users []UserItem

	// who's asking for the list?
	userUUID := c.Query("myUUID")
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
  http://localhost:3000/profilestats/9cf34d34-a042-4231-babc-eee6ba67bd18
  returns stats for a user
  {
	"following": 255,
	"followers": 12,
	"photos": 47,
	"email": "user@email.com",
	"username": "joesmith"
  }

 */
func getUserProfileStats(c *gin.Context) {
	var me User;

	myUUID := c.Param("myUUID")
	me, err := validateUser(myUUID)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, "user " + err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	var followerCount int = 0
	err = dbmap.SelectOne(&followerCount, "SELECT count(*) FROM follows WHERE UserID2=?", me.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	var followingCount int = 0
	err = dbmap.SelectOne(&followingCount, "SELECT count(*) FROM follows WHERE UserID1=?", me.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	var photoCount int = 0
	err = dbmap.SelectOne(&photoCount, "SELECT count(*) FROM photos WHERE UserID=?", me.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"following": followingCount,
		"followers": followerCount,
		"photos": photoCount,
		"email": me.Email,
		"username": me.Username,
	})
}

/* helper functions */

func fetchDoILikePhoto(myID uint, photoID uint) (bool, error) {
	var rowID int = 0
	err := dbmap.SelectOne(&rowID, `
		SELECT id
		FROM likes
		WHERE UserID=? AND PhotoID=?`, myID, photoID)
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

	err := dbmap.SelectOne(&rowID, `
		SELECT id
		FROM follows
		WHERE UserID1=? AND UserID2=?`, myID, userID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return false, err
	}
	if rowID > 0 {
		return true, nil
	}
	return false, nil
}

func fetchPhotoLikes(photoID uint) (int, error) {
	var count int = 0
	err := dbmap.SelectOne(&count, "SELECT count(*) FROM likes WHERE PhotoID=?", photoID)
	if err != nil {
		return -1, err
	}
	return count, nil
}

func validateUser(userUUID string) (User, error) {
	var data User
	if userUUID == "" {
		return data, errors.New("user UUID not set")
	}
	err := dbmap.SelectOne(&data, "SELECT * FROM users WHERE UUID=?", userUUID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("not found")
		}
		log.Println(err)
		return data, err
	}
	return data, nil
}

func validatePhoto(photoUUID string) (Photo, error) {
	var data Photo
	if photoUUID == "" {
		return data, errors.New("user UUID not set")
	}
	err := dbmap.SelectOne(&data, "SELECT * FROM photos WHERE UUID=?", photoUUID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("not found")
		}
		log.Println("validate photo err", err)
		return data, err
	}
	return data, nil
}

//func uniqueAppendString(slice []string, newString string) []string {
//	for _, ele := range slice {
//		if ele == newString {
//			return slice
//		}
//	}
//	return append(slice, newString)
//}

//func validateRow(strUUID string, table string, kind *interface{}) (error) {
//	if strUUID == "" {
//		return errors.New("not found")
//	}
//
//	err := dbmap.SelectOne(kind, "SELECT * FROM " + table + " WHERE UUID=?", strUUID)
//	if err != nil {
//		if err.Error() == "sql: no rows in result set" {
//			err = errors.New("not found")
//		}
//		return err
//	}
//	return nil
//}
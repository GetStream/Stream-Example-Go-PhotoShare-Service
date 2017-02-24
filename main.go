package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/GetStream/stream-go"

	// call our sub-packages so they'll run their init() files
	_ "github.com/GetStream/Stream-Example-Go-PhotoShare-Service/DB"
	_ "github.com/GetStream/Stream-Example-Go-PhotoShare-Service/Stream"
	_ "github.com/GetStream/Stream-Example-Go-PhotoShare-Service/S3"
	"github.com/GetStream/Stream-Example-Go-PhotoShare-Service/Stream"
	"github.com/GetStream/Stream-Example-Go-PhotoShare-Service/DB"
	"strings"
	"time"
	"github.com/pborman/uuid"
	"github.com/GetStream/Stream-Example-Go-PhotoShare-Service/Utils"
	"fmt"
	"os"
	"io"
	"github.com/GetStream/Stream-Example-Go-PhotoShare-Service/S3"
)

var router *gin.Engine

var StreamClient *getstream.Client

func main() {
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

	router.POST("/register", RegisterUser) // also does login

	router.GET("/users", Users)
	router.GET("/follow/:targetUUID", Follow)
	router.GET("/unfollow/:targetUUID", Unfollow)
	router.GET("/likephoto/:photoUUID", LikePhoto)
	router.GET("/unlikephoto/:photoUUID", UnlikePhoto)
	router.GET("/profilestats/:myUUID", UserProfileStats)

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
			statusCode, payload = Stream.FetchNotificationFeed(feedSlug, myUserUUID, lastActivityUUID)
		}
		c.JSON(statusCode, payload)
	})
	router.GET("/feed/user/:feedUUID", func(c *gin.Context) {
		feedStub := "user"
		feedUUID := c.Param("feedUUID")
		myUserUUID := c.Query("myUUID")

		lastActivityUUID := c.Query("newestActivityUUID")
		c.JSON(Stream.FetchFlatFeed(feedStub, feedUUID, myUserUUID, lastActivityUUID))
	})
	router.GET("/feed/timeline/:feedUUID", func(c *gin.Context) {
		feedStub := "timeline"
		feedUUID := c.Param("feedUUID")
		myUserUUID := c.Query("myUUID")

		lastActivityUUID := c.Query("newestActivityUUID")
		c.JSON(Stream.FetchFlatFeed(feedStub, feedUUID, myUserUUID, lastActivityUUID))
	})
	router.GET("/feed/timeline_aggregated/:feedUUID", func(c *gin.Context) {
		feedSlug := "timeline_aggregated"
		feedUUID := c.Param("feedUUID")
		myUserUUID := c.Query("myUUID")

		lastActivityUUID := c.Query("newestActivityUUID")
		c.JSON(Stream.FetchAggregatedFeed(feedSlug, feedUUID, myUserUUID, lastActivityUUID))
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

/* we took a shortcut on authentication where a user 'registering' with the same username/email
   already in the database would log in that user. This, of course, is not authentication best
   practice, but a proper auth workflow is outside the scope of this project.
*/
func RegisterUser(c *gin.Context) {
	var user DB.User
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
		var user DB.User
		err := DB.Map.SelectOne(&user, "SELECT * FROM users WHERE username=? AND email=?",
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
	err := DB.Map.SelectOne(&id, "SELECT id FROM users WHERE username=?", strings.ToLower(username))
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	if id > 0 {
		output = append(output, "username already in use")
	}
	err = DB.Map.SelectOne(&id, "SELECT id FROM users WHERE email=?", strings.ToLower(email))
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
	insert, err := DB.Map.Exec(`
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
func Users(c *gin.Context) {
	var data []DB.User
	var users []Stream.UserItem

	// who's asking for the list?
	userUUID := c.Query("myUUID")
	user, err := Utils.ValidateUser(userUUID)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "user UUID" + err.Error()})
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	_, err = DB.Map.Select(&data, "SELECT * FROM users ORDER BY username")
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	for _, oneUser := range data {
		doIFollow, _ := Utils.DoIFollowUser(user.ID, oneUser.ID)
		userItem := Stream.UserItem{
			UUID:      oneUser.UUID,
			Username:  oneUser.Username,
			Email:     oneUser.Email,
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
func UserProfileStats(c *gin.Context) {
	var me DB.User

	myUUID := c.Param("myUUID")
	me, err := Utils.ValidateUser(myUUID)
	if err != nil {
		if err.Error() == "not found" {
			c.JSON(http.StatusNotFound, "user "+err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	var followerCount int = 0
	err = DB.Map.SelectOne(&followerCount, "SELECT count(*) FROM follows WHERE UserID2=?", me.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	var followingCount int = 0
	err = DB.Map.SelectOne(&followingCount, "SELECT count(*) FROM follows WHERE UserID1=?", me.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	var photoCount int = 0
	err = DB.Map.SelectOne(&photoCount, "SELECT count(*) FROM photos WHERE UserID=?", me.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"following": followingCount,
		"followers": followerCount,
		"photos":    photoCount,
		"email":     me.Email,
		"username":  me.Username,
	})
}

/* best practice:
   my 'timeline' feed follows someone else's 'user' feed
*/
func Follow(c *gin.Context) {
	var follow_id uint = 0

	myUUID := c.Query("myUUID")
	me, err := Utils.ValidateUser(myUUID)
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
	target, err := Utils.ValidateUser(targetUUID)
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

	err = DB.Map.SelectOne(&follow_id, "SELECT id FROM follows WHERE UserID1=? AND UserID2=?", me.ID, target.ID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	var followUUID string = uuid.New()
	_, err = DB.Map.Exec(`INSERT INTO follows (UserID1, UserID2, UUID) VALUES (?, ?, ?)`, me.ID, target.ID, followUUID)
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
		Object:    fmt.Sprintf("user:%s", targetUUID),
		Actor:     fmt.Sprintf("user:%s", myUUID),
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
func Unfollow(c *gin.Context) {
	myUUID := c.Query("myUUID")
	me, err := Utils.ValidateUser(myUUID)
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
	target, err := Utils.ValidateUser(targetUUID)
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
	err = DB.Map.SelectOne(&foreign_uuid, `SELECT uuid FROM follows WHERE UserID1=? AND UserID2=?`, me.ID, target.ID)
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
	DB.Map.Exec("DELETE FROM follows WHERE UserID1=? AND UserID2=?", me.ID, target.ID)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func LikePhoto(c *gin.Context) {
	var targetUUID string

	myUUID := c.Query("myUUID")
	photoUUID := c.Param("photoUUID")

	user, err := Utils.ValidateUser(myUUID)
	if err != nil {
		if err.Error() == "not found" {
			log.Println("getLikePhoto, user uuid not found")
			c.JSON(http.StatusNotFound, "user "+err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	photo, err := Utils.ValidatePhoto(photoUUID)
	if err != nil {
		if err.Error() == "not found" {
			log.Println("getLikePhoto, photo uuid not found")
			c.JSON(http.StatusNotFound, "photo "+err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	var like DB.Likes
	DB.Map.SelectOne(&like, `SELECT ID,UUID,UserID,PhotoID FROM likes WHERE UserID=? AND PhotoID=? LIMIT 1`, user.ID, photo.ID)
	if like.ID > 0 {
		c.JSON(http.StatusOK, gin.H{"status": "you already like this"})
		return
	}

	err = DB.Map.SelectOne(&targetUUID, "SELECT uuid FROM users WHERE id=?", photo.UserID)
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
	likeDBPayload, err := DB.Map.Exec(`INSERT INTO likes (UserID, PhotoID, UUID, CreatedAt) VALUES (?, ?, ?, ?)`,
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
		Object:    fmt.Sprintf("photo:%s", photo.UUID),
		Actor:     fmt.Sprintf("user:%s", myUUID),
		MetaData: map[string]string{
			"photoUrl": photo.URL,
		},
	})
	if err != nil {
		log.Println("couldn't add activity to notification feed", err)
		c.JSON(http.StatusInternalServerError, gin.H{"errors": err.Error()})

		DB.Map.Exec("DELETE FROM likes WHERE ID=?", likeID)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func UnlikePhoto(c *gin.Context) {
	var targetUUID string

	myUUID := c.Query("myUUID")
	photoUUID := c.Param("photoUUID")

	user, err := Utils.ValidateUser(myUUID)
	if err != nil {
		if err.Error() == "not found" {
			log.Println("getLikePhoto, user uuid not found")
			c.JSON(http.StatusNotFound, "user "+err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	photo, err := Utils.ValidatePhoto(photoUUID)
	if err != nil {
		if err.Error() == "not found" {
			log.Println("getLikePhoto, photo uuid not found")
			c.JSON(http.StatusNotFound, "photo "+err.Error())
		} else {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, err.Error())
		}
		return
	}

	err = DB.Map.SelectOne(&targetUUID, "SELECT uuid FROM users WHERE id=?", photo.UserID)
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

	var like DB.Likes
	err = DB.Map.SelectOne(&like, `SELECT ID,UUID FROM likes WHERE UserID=? AND PhotoID=? LIMIT 1`, user.ID, photo.ID)
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

		_, err = DB.Map.Exec("DELETE FROM likes WHERE ID=?", like.ID)
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
	var me DB.User

	myUUID := c.PostForm("myUUID")
	me, err := Utils.ValidateUser(myUUID)
	if err != nil && err.Error() != "sql: no rows in result set" {
		log.Println("594", err.Error())
		c.JSON(http.StatusInternalServerError, "user "+err.Error())
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

	var photo DB.Photo
	photo.UUID = uuid.New()
	photo.UserID = me.ID

	insert, err := DB.Map.Exec(`
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

	go S3.Upload(myUUID, photo, localFilename, photo_id)
}

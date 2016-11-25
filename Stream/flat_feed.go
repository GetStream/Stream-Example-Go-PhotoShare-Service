package Stream

import (
	"github.com/GetStream/stream-go"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
	"github.com/GetStream/Stream-Example-Go-PhotoShare-Service/DB"
	"github.com/GetStream/Stream-Example-Go-PhotoShare-Service/Utils"
)

func FetchFlatFeed(
	feedSlug string,
	feedUserUUID string,
	myUserUUID string,
	lastActivityUUID string,
) (int, map[string]interface{}) {
	var err error
	var activities []FeedItem
	var newestActivityUUID string
	var me DB.User

	if feedUserUUID == "" {
		return http.StatusBadRequest, gin.H{"error": "user UUID not found"}
	}
	if feedUserUUID != "global" {
		_, err := Utils.ValidateUser(feedUserUUID)
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
		me, err = Utils.ValidateUser(myUserUUID)
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
	streamFeed, err := Client.FlatFeed(feedSlug, feedUserUUID)
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
		"uuid":               feedUserUUID,
		"newest_activity_id": newestActivityUUID,
		"feed":               activities,
	}
}

func parseFlatFeed(me DB.User, feedSlug string, inActivities []*getstream.Activity) []FeedItem {
	var activities []FeedItem
	var doIFollowUser bool = false
	var doILikePhoto bool = false

	for _, activity := range inActivities {
		bits := strings.Split(string(activity.Actor), ":")
		actorUUID := bits[1]
		user, err := Utils.ValidateUser(actorUUID)
		if err != nil {
			log.Println("skipping flat feed activity, activity author user lookup failed:", err.Error())
			log.Println("actor UUID:", actorUUID)
			continue
		}

		if me.ID > 0 {
			doIFollowUser, err = Utils.DoIFollowUser(me.ID, user.ID)
			if err != nil {
				log.Println("fetchDoIFollow error:", err)
				// TODO deal with database error?
			}
		} else if feedSlug == "timeline" {
			// you'd only be seeing this in your timeline if you're following them, so we'll force true
			doIFollowUser = true
		}

		photo, err := Utils.ValidatePhoto(activity.ForeignID)
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
			AuthorID:    user.UUID,
			AuthorName:  user.Username,
			Likes:       count,
			ILikeThis:   doILikePhoto,
			DoIFollow:   doIFollowUser,
			PhotoURL:    activity.MetaData["photoUrl"],
			PhotoUUID:   photo.UUID,
			ID:          activity.ForeignID,
			CreatedDate: activity.TimeStamp.Unix(),
		})
		log.Println(activities)
	}

	return activities
}


func fetchDoILikePhoto(myID uint, photoID uint) (bool, error) {
	var rowID int = 0
	err := DB.Map.SelectOne(&rowID, `
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

func fetchPhotoLikes(photoID uint) (int, error) {
	var count int = 0
	err := DB.Map.SelectOne(&count, "SELECT count(*) FROM likes WHERE PhotoID=?", photoID)
	if err != nil {
		return -1, err
	}
	return count, nil
}

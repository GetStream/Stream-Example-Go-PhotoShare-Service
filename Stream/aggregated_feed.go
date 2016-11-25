package Stream

import (
	"github.com/GetStream/stream-go"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
	"time"
	"github.com/GetStream/Stream-Example-Go-PhotoShare-Service/Utils"
)

func FetchAggregatedFeed(
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
		_, err = Utils.ValidateUser(myUserUUID)
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
	streamFeed, err := Client.AggregatedFeed(feedSlug, feedUserUUID)
	if err != nil {
		log.Println("fetch feed threw an error")
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}

	feedActivities, err := streamFeed.Activities(&options)
	activities := parseAggregatedFeed(feedActivities)

	newestActivityUUID = feedActivities.Next

	return http.StatusOK, gin.H{
		"uuid":               feedUserUUID,
		"newest_activity_id": newestActivityUUID,
		"feed":               activities,
	}
}

func parseAggregatedFeed(inActivities *getstream.GetAggregatedFeedOutput) []AggregatedFeedItem {
	activities := []AggregatedFeedItem{}

	for _, result := range inActivities.Results {

		groupBits := strings.Split(result.Group, "_")
		userBits := strings.Split(groupBits[0], ":")
		actorUUID := userBits[1]
		actor, err := Utils.ValidateUser(actorUUID)
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
			AuthorName:  actor.Username,
			AuthorID:    actor.UUID,
		}
		photos := []string{}
		for _, activity := range result.Activities {

			p, err := Utils.ValidatePhoto(activity.ForeignID)
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

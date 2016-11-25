package Stream

import (
	"github.com/GetStream/stream-go"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
	"github.com/GetStream/Stream-Example-Go-PhotoShare-Service/Utils"
)

func FetchNotificationFeed(
	feedSlug string,
	myUserUUID string,
	lastActivityUUID string,
) (int, map[string]interface{}) {
	var err error
	var newestActivityUUID string

	_, err = Utils.ValidateUser(myUserUUID)
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
	streamFeed, err := Client.NotificationFeed(feedSlug, myUserUUID)
	if err != nil {
		log.Println("fetch feed threw an error")
		return http.StatusInternalServerError, gin.H{"error": err.Error()}
	}

	feedActivities, err := streamFeed.Activities(&options)
	activities := parseNotificationFeed(feedActivities)

	//newestActivityUUID = feedActivities.Next

	return http.StatusOK, gin.H{
		"newest_activity_id": newestActivityUUID,
		"feed":               activities,
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
				actor, _ := Utils.ValidateUser(bits[1])
				if actor.ID <= 0 {
					continue
				}

				photoUrl := activity.MetaData["photoUrl"]
				if _, ok := likes[photoUrl]; !ok {
					likes[photoUrl] = []NotificationActor{}
				}
				likes[photoUrl] = append(likes[photoUrl], NotificationActor{
					AuthorEmail: actor.Email,
					AuthorName:  actor.Username,
				})
			}
		} else if verb == "follow" {
			for _, activity := range r.Activities {
				// who did this verb?
				bits := strings.Split(string(activity.Actor), ":")
				actor, _ := Utils.ValidateUser(bits[1])
				if actor.ID <= 0 {
					continue
				}
				follows = append(follows, NotificationActor{
					AuthorEmail: actor.Email,
					AuthorName:  actor.Username,
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
			Actors:   likes[photo_url],
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

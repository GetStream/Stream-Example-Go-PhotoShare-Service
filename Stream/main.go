package Stream

import (
	"os"
	"github.com/GetStream/stream-go"
	"log"
)

// Stream.io variables for our feeds
var GlobalFeed *getstream.FlatFeed
var AggregatedFeed *getstream.AggregatedFeed
var NotificationFeed *getstream.NotificationFeed

var Client *getstream.Client

func init()  {
	var err error

	log.Println("*********** STREAM SETUP BEGINNING ************")
	Client, err = getstream.New(&getstream.Config{
		APIKey:    os.Getenv("STREAM_API_KEY"),
		APISecret: os.Getenv("STREAM_API_SECRET"),
		AppID:     os.Getenv("STREAM_APP_ID"),
		Location:  os.Getenv("STREAM_REGION"),
	})
	if err != nil {
		panic("failed to connect to stream: " + err.Error())
	}

	GlobalFeed, err = Client.FlatFeed("user", "global")
	if err != nil {
		panic("could not set global feed")
	}

	AggregatedFeed, err = Client.AggregatedFeed("aggregated", "photos")
	if err != nil {
		panic("could not set aggregated feed")
	}

	NotificationFeed, err = Client.NotificationFeed("notification", "likes")
	if err != nil {
		panic("could not set notification feed")
	}
}

type NotificationActor struct {
	AuthorEmail string `json:"author_email"`
	AuthorName  string `json:"author_name"`
}

type NotificationLike struct {
	PhotoURL string              `json:"photo_url"`
	Actors   []NotificationActor `json:"actors"`
}

type UserItem struct {
	UUID      string `json:"uuid"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	DoIFollow bool   `json:"doifollow"`
}

type FeedItem struct {
	ID          string `json:"id"`
	AuthorEmail string `json:"author_email"`
	AuthorName  string `json:"author_name"`
	AuthorID    string `json:"author_id"`
	PhotoURL    string `json:"photo_url"`
	PhotoUUID   string `json:"photo_uuid"`
	DoIFollow   bool   `json:"doifollow"`
	Likes       int    `json:"likes"`
	ILikeThis   bool   `json:"ilikethis"`
	CreatedDate int64  `json:"created_date"`
}

type AggregatedFeedItem struct {
	AuthorEmail string   `json:"author_email"`
	AuthorName  string   `json:"author_name"`
	AuthorID    string   `json:"author_id"`
	Photos      []string `json:"photos"`
	CreatedDate int64    `json:"created_date"`
}

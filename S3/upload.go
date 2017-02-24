package S3

import (
	"github.com/disintegration/imaging"
	"os"
	"bytes"
	"net/http"
	"path"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"time"
	"github.com/aws/aws-sdk-go/service/s3"

	"fmt"
	"log"
	"github.com/GetStream/Stream-Example-Go-PhotoShare-Service/DB"
	"github.com/GetStream/Stream-Example-Go-PhotoShare-Service/Stream"
	"github.com/GetStream/stream-go"
)

func Upload(myUUID string, photo DB.Photo, localFilename string, photo_id int64) {
	//var photoFilename string

	// shrink image
	inImage, err := imaging.Open(localFilename)
	if err != nil {
		log.Println("Unknown image format, rejecting")
		return
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
	photo_path := "photos/" + path.Base(file.Name())
	params := &s3.PutObjectInput{
		Bucket:        aws.String(BucketName), // required
		Key:           aws.String(photo_path),   // required
		ACL:           aws.String("public-read"),
		Body:          fileBytes,
		ContentLength: &size,
		ContentType:   aws.String(fileType),
		Metadata: map[string]*string{
			"Key": aws.String("MetadataValue"), //required
		},
		// see more at http://godoc.org/github.com/aws/aws-sdk-go/service/s3#S3.PutObject
	}
	_, err = Client.PutObject(params)
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
	photo.URL = "https://android-demo.s3.amazonaws.com/" + photo_path
	_, err = DB.Map.Exec(`
			UPDATE photos SET URL=?, UpdatedAt=? WHERE ID=?`,
		photo.URL, now, photo_id)
	if err != nil {
		log.Println("sending error after photo insert")
		return
	}

	globalFeed, err := Stream.Client.FlatFeed("user", "global")
	if err != nil {
		log.Println(err)
	}
	userFeed, err := Stream.Client.FlatFeed("user", myUUID)
	if err != nil {
		log.Println(err)
	} else {
		_, err = globalFeed.AddActivity(&getstream.Activity{
			Verb:      "photo",
			ForeignID: photo.UUID,
			TimeStamp: &now,
			To:        []getstream.Feed{userFeed},
			Object:    fmt.Sprintf("photo:%s", photo.UUID),
			Actor:     fmt.Sprintf("user:%s", myUUID),
			MetaData: map[string]string{
				// add as many custom keys/values here as you like
				"photoUrl": photo.URL,
			},
		})
		if err != nil {
			fmt.Println(err)
		}
	}
}

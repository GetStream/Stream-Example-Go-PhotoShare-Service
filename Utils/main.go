package Utils
// helper functions

import (
	"github.com/GetStream/Stream-Example-Go-PhotoShare-Service/DB"
	"errors"
	"log"
)

func ValidateUser(userUUID string) (DB.User, error) {
	var data DB.User
	if userUUID == "" {
		return data, errors.New("user UUID not set")
	}
	err := DB.Map.SelectOne(&data, "SELECT * FROM users WHERE UUID=?", userUUID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("not found")
		}
		log.Println(err)
		return data, err
	}
	return data, nil
}

func ValidatePhoto(photoUUID string) (DB.Photo, error) {
	var data DB.Photo
	if photoUUID == "" {
		return data, errors.New("user UUID not set")
	}
	err := DB.Map.SelectOne(&data, "SELECT * FROM photos WHERE UUID=?", photoUUID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			err = errors.New("not found")
		}
		log.Println("validate photo err", err)
		return data, err
	}
	return data, nil
}

func DoIFollowUser(myID uint, userID uint) (bool, error) {
	var rowID int = 0

	err := DB.Map.SelectOne(&rowID, `
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

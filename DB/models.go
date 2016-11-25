package DB

import "github.com/jinzhu/gorm"

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

type Photo struct {
	gorm.Model
	UserID uint   `json:"user_id"`
	UUID   string `json:"uuid"`
	URL    string `json:"url"`
	Likes  int    `json:"likes"`
}
type Follows struct {
	gorm.Model
	UserID1 uint   `json:"user_id_1"`
	UserID2 uint   `json:"user_id_2"`
	UUID    string `json:"uuid"`
}


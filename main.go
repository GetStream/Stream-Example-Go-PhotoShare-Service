package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"io/ioutil"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/antonholmquist/jason"
	"golang.org/x/oauth2"
	"github.com/pborman/uuid"
	"database/sql"
	//_ "github.com/mattn/go-sqlite3"
)

type AccessToken struct {
	Token  string
	Expiry time.Time
}

type User struct {
	ID          int64
	UUID        string
	Username    string
	FacebookID  string
	FacebookImg string
	UserToken   UserToken
}

type UserToken struct {
	Token  string
	Expiry time.Time
}

var router *gin.Engine
var DB *sql.DB

func main() {
	// "mobile-backend.db"
	DB, err := sql.Open("sqlite3", "/tmp/getstream-mobile-backend.db")
	if err != nil {
		panic("failed to connect database")
	}
	defer DB.Close()

	gin.SetMode(gin.DebugMode)
	router = gin.Default()
	router.LoadHTMLGlob("templates/*")

	router.GET("/", func(c *gin.Context) {
		// redirect to the repo, blog post, etc.
		c.Redirect(http.StatusTemporaryRedirect, "//github.com/GetStream")
	})
	router.GET("/_ah_health", func(c *gin.Context) {
		// redirect to the repo, blog post, etc.
		c.HTML(http.StatusOK, "healthcheck.html", gin.H{"title": "ok"})
	})
	router.GET("/privacy", func(c *gin.Context) {
		// redirect to the repo, blog post, etc.
		c.HTML(http.StatusOK, "privacy.html", gin.H{})
	})
	router.GET("/termsofservice", func(c *gin.Context) {
		// redirect to the repo, blog post, etc.
		c.HTML(http.StatusOK, "termsofservice.html", gin.H{})
	})
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	//////// custom endpoints here

	router.GET("/user/:username", func(c *gin.Context) {
		name := c.Param("username")
		// get user stream, send back in json format
		c.JSON(http.StatusOK, gin.H{
			"username": name,
		})
	})

	router.GET("/follow/:source/:target", func(c *gin.Context) {
		//sourceFeedName := c.Param("sourceName")
		//targetFeedName := c.Param("targetName")

		// validate that sourceUuid and targetUuid are valid
		// source follows target, pull 100 items into their feed

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
		})
	})

	router.GET("/register", fbRegister)
	router.GET("/FBLogin", fbLogin)
	router.POST("/upload", process_upload)

	///////// no more custom code under here
	log.Print("Listening on port 8080")
	router.Run()
}

func process_upload(c *gin.Context) {
	file, header, err := c.Request.FormFile("upload")
	filename := header.Filename
	fmt.Println(header.Filename)
	out, err := os.Create("./tmp/" + filename + ".png")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		log.Fatal(err)
	}
}

var fbConfig = &oauth2.Config{
	// ClientId: FBAppID(string), ClientSecret : FBSecret(string)
	// Example - ClientId: "1234567890", ClientSecret: "red2drdff6e2321e51aedcc94e19c76ee"

	ClientID:     "196848327415282",
	ClientSecret: "903969cabc53ba4d51fbb81fa36d310e",
	RedirectURL:  "http://getstream-mobile.appspot.com/FBLogin",
	Scopes:       []string{"email", "user_about_me"},
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https://www.facebook.com/dialog/oauth",
		TokenURL: "https://graph.facebook.com/oauth/access_token",
	},
}

func fbRegister(c *gin.Context) {
	url := fbConfig.AuthCodeURL("")
	c.HTML(http.StatusOK, "facebook.html", gin.H{"facebook_url": url})
}

func fbLogin(c *gin.Context) {
	code := c.Query("code")
	log.Println("incoming code: " + code)

	accessToken, err := fbConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	fbCheckLogin(c, accessToken, "")
}

func fbCheckLogin(c *gin.Context, accessToken *oauth2.Token, username string) {
	var user *User
	log.Println("accessToken:", accessToken)
	log.Println("username:", username)

	if username != "" {
		log.Println("looking up username", username)
		rows, err := DB.Query(`
			SELECT u.id, u.uuid. u.facebook_id, u.username, u.profile_image, ut.token, ut.expiry
			FROM users u
			JOIN user_tokens ut ON u.id=ut.uid
			LIMIT 1
			`)
		if err != nil {
			log.Println("error getting user details for username", username)
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		for rows.Next() {
			err = rows.Scan(
				&user.ID,
				&user.UUID,
				&user.FacebookID,
				&user.Username,
				&user.FacebookImg,
				&user.UserToken.Token,
				&user.UserToken.Expiry,
			)
			log.Println(user)
			if err != nil {
				log.Println("error putting user details into struct")
				c.JSON(http.StatusInternalServerError, err.Error())
				return
			}
		}
	}

	log.Println("pinging facebook with access token")
	response, err := http.Get("https://graph.facebook.com/me?access_token=" + accessToken.AccessToken)
	if err != nil {
		log.Println("error getting graph using access token")
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Println("error reading response from Facebook")
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	// see https://www.socketloop.com/tutorials/golang-process-json-data-with-jason-package
	userPayload, _ := jason.NewObjectFromBytes([]byte(contents))

	user.Username, err = userPayload.GetString("username")
	if err != nil {
		log.Println(err)
	}
	user.FacebookID, err = userPayload.GetString("id")
	if err != nil {
		log.Println(err)
	}
	user.FacebookImg = "https://graph.facebook.com/" + user.FacebookID + "/picture?width=180&height=180"

	log.Println("new user:", user)
	if username == "" {
		log.Println("saving new user in db")
		// save user details in the db
		stmt, err := DB.Prepare(`
				INSERT INTO users (uuid, username, facebook_id, profile_image, created_at, updated_at)
				VALUES (?,?,?,?,?,?,?)`,
		)
		if err != nil {
			panic(fmt.Sprintf("could create user insert statement, %v", err))
		}
		res, err := stmt.Exec(
			uuid.New(),
			user.Username,
			user.FacebookID,
			user.FacebookImg,
			time.Now(),
			time.Now(),
		)
		if err != nil {
			panic("failed to run user insert statement")
		}

		affect, err := res.RowsAffected()
		if err != nil {
			panic("failed to run rows affected after user insert")
		}
		log.Println("rows affected:", affect)

		user.ID, err = res.LastInsertId()
		if err != nil {
			panic("failed to get last_insert_id")
		}

		stmt, err = DB.Prepare("INSERT INTO user_facebook_token (uid, created_at, updated_at) VALUES (?,?,?)")
		if err != nil {
			panic(fmt.Sprintf("could create token insert statement, %v", err))
		}
		res, err = stmt.Exec(user.ID, time.Now(), time.Now())
		if err != nil {
			panic("failed to run token insertstatement")
		}

		affect, err = res.RowsAffected()
		if err != nil {
			panic("failed to run rows affected after user token insert")
		}
		log.Println("rows affected:", affect)

		user.UserToken.Token = accessToken.AccessToken
		user.UserToken.Expiry = accessToken.Expiry
	} else {
		stmt, err := DB.Prepare(`UPDATE users SET username=?, profile_image=?, updated_at=? WHERE facebook_id=?`,
		)
		if err != nil {
			panic(fmt.Sprintf("could create user update statement, %v", err))
		}
		res, err := stmt.Exec(
			user.Username,
			user.FacebookImg,
			user.FacebookID,
			time.Now(),
		)
		if err != nil {
			panic("failed to run user insert statement")
		}
		affect, err := res.RowsAffected()
		if err != nil {
			panic("failed to run rows affected after user update")
		}
		log.Println("rows affected:", affect)
	}
	stmt, err := DB.Prepare("UPDATE user_facebook_token SET token=?, expiry=? WHERE uid=?")
	if err != nil {
		panic(fmt.Sprintf("could create token update statement, %v", err))
	}
	res, err := stmt.Exec(user.UserToken.Token, user.UserToken.Expiry, user.ID)
	if err != nil {
		panic("failed to run statement")
	}
	affect, err := res.RowsAffected()
	if err != nil {
		panic("failed to run rows affected after user token update")
	}
	log.Println("rows affected:", affect)

	c.HTML(http.StatusOK, "facebook_user.html", gin.H{
		"fb_id": user.FacebookID,
		"username": user.Username,
		"fb_img": user.FacebookImg,
	})
	// see https://www.socketloop.com/tutorials/golang-download-file-example on how to save FB file to disk
}

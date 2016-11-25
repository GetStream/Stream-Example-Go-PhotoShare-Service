package DB

import (
	"gopkg.in/gorp.v1"
	_ "github.com/go-sql-driver/mysql"
	"database/sql"
)

var Map *gorp.DbMap

func init() {
	db, err := sql.Open("mysql", "stream:B4ck3nd!@/stream_backend?parseTime=true")
	if err != nil || db == nil {
		panic("failed to connect database: " + err.Error())
	}
	Map = &gorp.DbMap{Db: db, Dialect: gorp.MySQLDialect{"InnoDB", "UTF8"}}
	Map.AddTableWithName(User{}, "users").SetKeys(true, "ID")
	Map.AddTableWithName(Photo{}, "photos").SetKeys(true, "ID")
	Map.AddTableWithName(Likes{}, "likes").SetKeys(true, "ID")
	Map.AddTableWithName(Follows{}, "follows").SetKeys(true, "ID")
	err = Map.CreateTablesIfNotExists()
	if err != nil {
		panic("failed to create tables: " + err.Error())
	}
}
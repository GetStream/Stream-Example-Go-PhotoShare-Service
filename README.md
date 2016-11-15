## Go backend for GetStream mobile app

We built this project as an example of our "best practices" for building a photo-sharing mobile app like Instagram.

Our [blog post]() covers more of the background of the application, and we wanted this README to cover more of the technical setup involved.

## Environment

We developed this project with Go 1.7, MySQL and a handful of open-source Go libraries.

```
# SDK for Stream API:
go install github.com/GetStream/stream-go

# used for image manipulation
go install github.com/disintegration/imaging

# Web framework we used to develop the API service
go install github.com/gin-gonic/gin

# used for generating unique collision-resistant UUIDs
go install github.com/pborman/uuid

# ORMs we used in the project
go install github.com/jinzhu/gorm
go install gopkg.in/gorp.v1

# Go library for talking to MySQL
go install github.com/go-sql-driver/mysql

# Amazon libraries for uploading images to an S3 bucket
go install github.com/aws/aws-sdk-go/aws
go install github.com/aws/aws-sdk-go/service/s3
```

## Environment

To run the server, you will need an environment set up with the following variables:

```
# these can be found on your Stream dashboard
export STREAM_API_KEY=
export STREAM_API_SECRET=
export STREAM_REGION=
export STREAM_APP_ID=

# these are issued by Amazon
export AWS_ACCESS_KEY_ID=
export AWS_SECRET_ACCESS_KEY=
```

The application will not operate properly without these values

## Stream Setup

You will need to create the following feeds:

- flat feed called 'user'
- flat feed called 'timeline'
- aggregated feed called 'timeline_aggregated' with an aggregation formation of `{{actor}}_{{ verb.id }}_{{ time.strftime('%Y-%m-%d') }}`
- notification feed called 'notification'

## Database Setup

main.go should generate the tables you need to get set up with the application, but it if you get stuck, there's a MySQL schema in schema.sql
which contains a simple database structure for the application. **CAVEAT**: this schema.sql is not set up for efficient indexing; you should 
always follow best practices for your database engine to set up proper indexing, unique keys and foreign keys based on your application. At
small scale, this schema will work, but is not suitable for production use.

You will need to modify the connection string for your database by replacing these line with an appropriate username, password, hostname, 
and database name for your environment, and the dialect setup:
```go
db, err := sql.Open("mysql", "stream:B4ck3nd!@/stream_backend?parseTime=true")
...
dbmap := &gorp.DbMap{Db: db, Dialect: gorp.MySQLDialect{"InnoDB", "UTF8"}}
```

## S3 Setup

The S3 setup in our code assumes you have made a bucket called "photos" within the "us-east-1" region. You will need to adjust main.go
according to your preferences

## Testing the backend API

We will be publishing a Postman export with this repo soon to assist with testing the API service.

## Contributing

As with all of our open source projects, we love to hear from our community and look forward to your feedback on this best-practices project.
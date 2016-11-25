package S3
import (
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws"
)

var BucketName string = "android-demo"
var Client *s3.S3

func init()  {
	Client = s3.New(session.New(&aws.Config{Region: aws.String("us-east-1")}))
}
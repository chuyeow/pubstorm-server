package s3client

import (
	"io"
	"math"
	"os"
	"time"

	"github.com/nitrous-io/rise-server/pkg/filetransfer"
)

var (
	BucketRegion = os.Getenv("S3_BUCKET_REGION")
	BucketName   = os.Getenv("S3_BUCKET_NAME")

	MaxUploadSize = int64(1024 * 1024 * 1000) // 1 GiB
	PartSize      = int64(50 * 1024 * 1024)   // 50 MiB

	MaxUploadParts = int(math.Ceil(float64(MaxUploadSize) / float64(PartSize)))

	S3 filetransfer.FileTransfer = filetransfer.NewS3(PartSize, MaxUploadParts)
)

func init() {
	if BucketRegion == "" {
		BucketRegion = "us-west-2"
	}

	if BucketName == "" {
		BucketName = "rise-development-usw2"
	}
}

func Upload(path string, body io.Reader, contentType, acl string) error {
	return S3.Upload(BucketRegion, BucketName, path, body, contentType, acl)
}

func Download(path string, out io.WriterAt) error {
	return S3.Download(BucketRegion, BucketName, path, out)
}

func Delete(path ...string) error {
	return S3.Delete(BucketRegion, BucketName, path...)
}

func Copy(src, dest string) error {
	return S3.Copy(BucketRegion, BucketName, src, dest)
}

func Exists(path string) (bool, error) {
	return S3.Exists(BucketRegion, BucketName, path)
}

func PresignedURL(key string, expireTime time.Duration) (string, error) {
	return S3.PresignedURL(BucketRegion, BucketName, key, expireTime)
}

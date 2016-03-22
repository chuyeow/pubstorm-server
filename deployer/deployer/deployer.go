package deployer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nitrous-io/rise-server/pkg/filetransfer"
	"github.com/nitrous-io/rise-server/pkg/pubsub"
	"github.com/nitrous-io/rise-server/shared/exchanges"
	"github.com/nitrous-io/rise-server/shared/messages"
	"github.com/nitrous-io/rise-server/shared/s3"
)

func init() {
	riseEnv := os.Getenv("RISE_ENV")
	if riseEnv == "" {
		riseEnv = "development"
	}

	if riseEnv != "test" {
		if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
			log.Fatal("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables are required!")
		}
	}
}

var S3 filetransfer.FileTransfer = filetransfer.NewS3(s3.PartSize, s3.MaxUploadParts)

func Work(data []byte) error {
	d := &messages.DeployJobData{}
	err := json.Unmarshal(data, d)
	if err != nil {
		return err
	}

	prefix := fmt.Sprintf("%s-%d", d.DeploymentPrefix, d.DeploymentID)
	rawBundle := "deployments/" + prefix + "/raw-bundle.tar.gz"
	tmpFileName := prefix + "-raw-bundle.tar.gz"

	f, err := ioutil.TempFile("", tmpFileName)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	if err := S3.Download(s3.BucketRegion, s3.BucketName, rawBundle, f); err != nil {
		return err
	}

	gr, err := gzip.NewReader(f)
	if err != nil {
		fmt.Println("could not unzip", err)
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	webroot := "deployments/" + prefix + "/webroot"

	// webroot is publicly readable
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if hdr.FileInfo().IsDir() {
			continue
		}

		fileName := path.Clean(hdr.Name)
		remotePath := webroot + "/" + fileName

		contentType := mime.TypeByExtension(filepath.Ext(fileName))
		if i := strings.Index(contentType, ";"); i != -1 {
			contentType = contentType[:i]
		}

		if err := S3.Upload(s3.BucketRegion, s3.BucketName, remotePath, tr, contentType, "public-read"); err != nil {
			return err
		}
	}

	// the metadata file is also publicly readable, do not put sensitive data
	metaJson, err := json.Marshal(map[string]interface{}{
		"prefix": prefix,
	})
	if err != nil {
		return err
	}

	reader := bytes.NewReader(metaJson)

	for _, domain := range d.Domains {
		reader.Seek(0, 0)
		if err := S3.Upload(s3.BucketRegion, s3.BucketName, "domains/"+domain+"/meta.json", reader, "application/json", "public-read"); err != nil {
			return err
		}
	}

	m, err := pubsub.NewMessageWithJSON(exchanges.Edges, exchanges.RouteV1Invalidation, &messages.V1InvalidationMessageData{
		Domains: d.Domains,
	})

	if err := m.Publish(); err != nil {
		return err
	}

	return nil
}
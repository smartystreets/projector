package persist

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/smartystreets/logging"
	"github.com/smartystreets/s3"
)

type S3Reader struct {
	logger *logging.Logger

	storage     s3.Option
	credentials s3.Option
	client      HTTPClient
}

// temporary function for compatibility
func NewDocumentReader(storageAddress *url.URL, accessKey, secretKey string, client HTTPClient) Reader {
	return NewS3Reader(storageAddress, accessKey, secretKey, client)
}

func NewS3Reader(storageAddress *url.URL, accessKey, secretKey string, client HTTPClient) *S3Reader {
	return &S3Reader{
		storage:     s3.StorageAddress(storageAddress),
		credentials: s3.Credentials(accessKey, secretKey),
		client:      client,
	}
}

func (this *S3Reader) Read(path string, document interface{}) error {
	request, err := s3.NewRequest(s3.GET, this.credentials, this.storage, s3.Key(path))
	if err != nil {
		return fmt.Errorf("Could not create signed request: '%s'", err.Error())
	}

	response, err := this.client.Do(request)
	if err != nil {
		return fmt.Errorf("HTTP Client Error: '%s'", err.Error())
	}

	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusNotFound {
		this.logger.Printf("[INFO] Document not found at '%s'\n", path)
		return nil
	}

	reader := response.Body.(io.Reader)
	if response.Header.Get("Content-Encoding") == "gzip" {
		reader, _ = gzip.NewReader(reader)
	}

	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(document); err != nil {
		return fmt.Errorf("Document read error: '%s'", err.Error())
	}

	return nil
}

func (this *S3Reader) ReadPanic(path string, document interface{}) {
	if err := this.Read(path, document); err != nil {
		this.logger.Panic(err)
	}
}
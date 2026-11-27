package httptransport

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type gzipReadCloser struct {
	*gzip.Reader
	source io.ReadCloser
}

func (body *gzipReadCloser) Close() error {
	return errors.Join(body.Reader.Close(), body.source.Close())
}

type errorReadCloser struct {
	source io.ReadCloser
	err    error
}

func (body *errorReadCloser) Read([]byte) (int, error) {
	return 0, body.err
}

func (body *errorReadCloser) Close() error {
	return body.source.Close()
}

// NormalizeResponseEncoding transparently decodes a gzip response and updates
// its metadata to describe the decoded body.
func NormalizeResponseEncoding(response *http.Response) error {
	if response == nil || response.Body == nil || response.Body == http.NoBody {
		return nil
	}
	encodings := response.Header.Values("Content-Encoding")
	if len(encodings) == 0 {
		return nil
	}

	var nonIdentity []string
	for _, value := range encodings {
		for _, token := range strings.Split(value, ",") {
			token = strings.TrimSpace(token)
			if token == "" || strings.EqualFold(token, "identity") {
				continue
			}
			nonIdentity = append(nonIdentity, token)
		}
	}
	if len(nonIdentity) == 0 {
		response.Header.Del("Content-Encoding")
		return nil
	}
	if len(nonIdentity) != 1 || !strings.EqualFold(nonIdentity[0], "gzip") {
		return setResponseEncodingError(
			response,
			fmt.Errorf("unsupported content encoding %q", strings.Join(nonIdentity, ", ")),
		)
	}

	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		return setResponseEncodingError(response, err)
	}
	response.Body = &gzipReadCloser{Reader: reader, source: response.Body}
	response.Header.Del("Content-Encoding")
	response.Header.Del("Content-Length")
	response.ContentLength = -1
	response.Uncompressed = true
	return nil
}

func setResponseEncodingError(response *http.Response, err error) error {
	response.Body = &errorReadCloser{source: response.Body, err: err}
	return err
}

package httptransport

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type closeTrackingBody struct {
	io.Reader
	closes int
}

func (body *closeTrackingBody) Close() error {
	body.closes++
	return nil
}

func TestNormalizeResponseEncodingDecodesGzipAndUpdatesMetadata(t *testing.T) {
	source := &closeTrackingBody{Reader: bytes.NewReader(gzipBytes(t, "decoded"))}
	response := &http.Response{
		Header: http.Header{
			"Content-Encoding": {"GZip, identity"},
			"Content-Length":   {"100"},
		},
		Body:          source,
		ContentLength: 100,
	}

	if err := NormalizeResponseEncoding(response); err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if string(content) != "decoded" {
		t.Fatalf("content = %q", content)
	}
	if source.closes != 1 {
		t.Fatalf("source closes = %d, want 1", source.closes)
	}
	if encoding := response.Header.Get("Content-Encoding"); encoding != "" {
		t.Fatalf("content encoding = %q", encoding)
	}
	if length := response.Header.Get("Content-Length"); length != "" {
		t.Fatalf("content length header = %q", length)
	}
	if response.ContentLength != -1 || !response.Uncompressed {
		t.Fatalf("response metadata = length %d, uncompressed %t", response.ContentLength, response.Uncompressed)
	}
}

func TestNormalizeResponseEncodingRejectsUnsupportedCombination(t *testing.T) {
	source := &closeTrackingBody{Reader: strings.NewReader("encoded")}
	response := &http.Response{
		Header: http.Header{"Content-Encoding": {"gzip, br"}},
		Body:   source,
	}
	normalizeErr := NormalizeResponseEncoding(response)
	if normalizeErr == nil {
		t.Fatal("expected unsupported encoding error")
	}
	if _, err := io.ReadAll(response.Body); !errors.Is(err, normalizeErr) {
		t.Fatalf("body read error = %v, want %v", err, normalizeErr)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if source.closes != 1 {
		t.Fatalf("source closes = %d, want 1", source.closes)
	}
}

func TestNormalizeResponseEncodingRejectsMalformedGzip(t *testing.T) {
	response := &http.Response{
		Header: http.Header{"Content-Encoding": {"gzip"}},
		Body:   io.NopCloser(strings.NewReader("not gzip")),
	}
	if err := NormalizeResponseEncoding(response); err == nil {
		t.Fatal("expected malformed gzip error")
	}
}

func gzipBytes(t *testing.T, value string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write([]byte(value)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

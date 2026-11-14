package aws

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// serve starts a server that answers the index and the s3 document from testdata. The index URLs point
// back at the server, as the real index's do.
func serve(t *testing.T) *httptest.Server {
	t.Helper()
	index, err := os.ReadFile("testdata/index.json")
	if err != nil {
		t.Fatal(err)
	}
	s3, err := os.ReadFile("testdata/s3.json")
	if err != nil {
		t.Fatal(err)
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(strings.ReplaceAll(string(index), "{BASE}", srv.URL)))
		case "/v1/s3/s3.json":
			_, _ = w.Write(s3)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestIndexListsServices(t *testing.T) {
	srv := serve(t)
	entries, err := Client{HTTP: srv.Client(), IndexURL: srv.URL + "/"}.Index(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("index = %+v", entries)
	}
	var s3 IndexEntry
	for _, e := range entries {
		if e.Service == "s3" {
			s3 = e
		}
	}
	if s3.ServiceName != "S3" || !strings.HasPrefix(s3.URL, srv.URL) {
		t.Errorf("s3 entry = %+v, want ServiceName S3 under the test server", s3)
	}
}

func TestServiceDocumentParsesActionsAndAnnotations(t *testing.T) {
	srv := serve(t)
	c := Client{HTTP: srv.Client()}
	doc, err := c.Service(t.Context(), srv.URL+"/v1/s3/s3.json")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name != "s3" || doc.ServiceName != "S3" || len(doc.Actions) != 3 {
		t.Fatalf("doc = %s/%s with %d actions", doc.Name, doc.ServiceName, len(doc.Actions))
	}
	abort := doc.Actions[0]
	if abort.Name != "AbortMultipartUpload" || !abort.Annotations.Properties.IsWrite || !abort.Annotations.Properties.IsDataPlane {
		t.Errorf("AbortMultipartUpload annotations = %+v", abort.Annotations.Properties)
	}
}

func TestHTTPErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := Client{HTTP: srv.Client(), IndexURL: srv.URL}.Index(t.Context())
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want a 404", err)
	}
}

func TestOversizedDocumentIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte(" "), maxDoc+1))
	}))
	defer srv.Close()
	_, err := Client{HTTP: srv.Client(), IndexURL: srv.URL}.Index(t.Context())
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err = %v, want an explicit size error", err)
	}
}

func TestNilHTTPUsesDefaultClient(t *testing.T) {
	srv := serve(t)
	entries, err := Client{IndexURL: srv.URL + "/"}.Index(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2", len(entries))
	}
}

func TestTransientFailuresAreRetried(t *testing.T) {
	old := retryBase
	retryBase = time.Millisecond
	defer func() { retryBase = old }()

	index, err := os.ReadFile("testdata/index.json")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls <= 2 {
			http.Error(w, "slow down", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(bytes.ReplaceAll(index, []byte("{BASE}"), []byte("http://x")))
	}))
	defer srv.Close()

	entries, err := Client{HTTP: srv.Client(), IndexURL: srv.URL}.Index(t.Context())
	if err != nil {
		t.Fatalf("Index should succeed after retries: %v", err)
	}
	if calls != 3 || len(entries) != 2 {
		t.Errorf("calls = %d, entries = %d; want 3 calls and 2 entries", calls, len(entries))
	}
}

func TestClientErrorsAreNotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := (Client{HTTP: srv.Client(), IndexURL: srv.URL}).Index(t.Context()); err == nil {
		t.Fatal("want an error")
	}
	if calls != 1 {
		t.Errorf("a 404 was attempted %d times, want once", calls)
	}
}

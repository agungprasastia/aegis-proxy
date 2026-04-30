package testhelpers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "modernc.org/sqlite"
)

func SetupTestDB() (*sql.DB, func()) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(fmt.Sprintf("open test db: %v", err))
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		panic(fmt.Sprintf("enable foreign keys: %v", err))
	}

	return db, func() { _ = db.Close() }
}

func NewTestRequest(method, path string, body interface{}) *http.Request {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			panic(fmt.Sprintf("marshal request body: %v", err))
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, path, reader)
	if err != nil {
		panic(fmt.Sprintf("new test request: %v", err))
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func AssertStatusCode(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Fatalf("status code = %d, want %d", resp.StatusCode, expected)
	}
}

func AssertJSONField(t *testing.T, body []byte, field string, expected interface{}) {
	t.Helper()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("unmarshal JSON body: %v", err)
	}
	got, ok := data[field]
	if !ok {
		t.Fatalf("JSON field %q missing", field)
	}
	if !jsonEqual(got, expected) {
		t.Fatalf("JSON field %q = %#v, want %#v", field, got, expected)
	}
}

func LoadFixture(filename string) []byte {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("locate testhelpers package")
	}
	path := filepath.Join(filepath.Dir(currentFile), "fixtures", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("load fixture %q: %v", filename, err))
	}
	return data
}

func jsonEqual(got, expected interface{}) bool {
	gotJSON, err := json.Marshal(got)
	if err != nil {
		return false
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return false
	}
	return bytes.Equal(gotJSON, expectedJSON)
}

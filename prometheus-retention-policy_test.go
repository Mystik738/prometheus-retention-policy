package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRunPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/v1/label/__name__/values" {
			rw.WriteHeader(http.StatusOK)
			fmt.Fprintf(rw, "{\"status\": \"success\",\"data\": [\"series_a\", \"series_b\"]}")
		} else if req.URL.Path == "/api/v1/admin/tsdb/delete_series" {
			rw.WriteHeader(http.StatusNoContent)
		} else if req.URL.Path == "/api/v1/admin/tsdb/clean_tombstones" {
			rw.WriteHeader(http.StatusNoContent)
		} else {
			log.Println(req.URL.String())
			rw.WriteHeader(http.StatusNotFound)
		}

	}))

	defer server.Close()

	os.Setenv("PROMETHEUS_URL", server.URL)
	os.Setenv("POLICY", "{\"set_default\":true, \"default\": 31536000,\"retentions\":[{\"seconds\":86400,\"metrics\":[\"series_a\"]}]} ")

	runPolicy()
}

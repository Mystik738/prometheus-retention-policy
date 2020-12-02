package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRunPolicy(t *testing.T) {
	os.Setenv("PROMETHEUS_URL", "http://localhost")
	os.Setenv("POLICY", "{\"default\": 31536000,\"retentions\":[{\"seconds\":86400,\"metrics\":[]}]} ")

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.String() == "/api/v1/label/__name__/values" {
			rw.WriteHeader(http.StatusOK)
			fmt.Fprintf(rw, "{\"status\": \"success\",\"data\": []}")
		}

		if req.URL.String() == "api/v1/admin/tsdb/delete_series" {
			rw.WriteHeader(http.StatusNoContent)
		}

	}))

	defer server.Close()

	runPolicy()
}

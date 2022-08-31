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

	os.Setenv("SOURCE_URL", server.URL)
	os.Setenv("POLICY", "{\"set_default\":true, \"default\": 31536000,\"retentions\":[{\"seconds\":86400,\"metrics\":[\"series_a\"]}]} ")

	runPolicy()
}

// TODO: This test needs a lot of work, but testing this might mean checking in a prometheus
func TestMinio(t *testing.T) {
	os.Setenv("SOURCE_URL", "minio.home")
	os.Setenv("SOURCE_BUCKET", "thanos")
	os.Setenv("SOURCE_USERNAME", "")
	os.Setenv("SOURCE_PASSWORD", "")
	os.Setenv("SOURCE_TYPE", "minio")
	os.Setenv("POLICY", `{"retentions":[{"seconds":315360000,"metrics":["power_factor_ratio","frequency","amperage","voltage","apparent_power","reactive_power","active_power","watt_hours_lifetime","watt_hours_seven_days","watt_hours_today","total_watts","reported_watts","aqi10", "aqi25", "pm10", "pm25", "pressure", "temperature","humidity","rpi_cpu_temperature_celsius","temperature_bed_actual", "temperature_bed_target", "temperature_tool0_actual","temperature_tool0_target"]}],"default":1209600,"set_default":true}`)
	os.Setenv("LOG_LEVEL", "debug")

	runPolicy()
}

package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	timeLayout  = "2006-01-02T15:04:05Z"
	concurrency = 8
)

type retention struct {
	Metrics []string `json:"metrics"`
	Seconds int      `json:"seconds"`
}

type policy struct {
	Retentions     []retention `json:"retentions"`
	DefaultSeconds int         `json:"default"`
	SetDefault     bool        `json:"set_default"`
}

type promNameValues struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

func main() {
	runPolicy()
}

func runPolicy() {
	prometheusURL, ok := os.LookupEnv("PROMETHEUS_URL")
	exitIf(!ok, "No URL set, set environment variable PROMETHEUS_URL")
	policyJSON, ok := os.LookupEnv("POLICY")
	exitIf(!ok, "No policy set, set environment variable POLICY")

	var policy policy
	err := json.Unmarshal([]byte(policyJSON), &policy)
	exitIf(err != nil, "Policy is not valid JSON policy")

	currentTime := time.Now()
	log.Infof("Beginning retention sweep on %v at %v", prometheusURL, currentTime.Format(timeLayout))

	//Get all metrics from endpoint
	client := &http.Client{
		Timeout: 600 * time.Second,
	}

	log.Infof("Retrieving all metrics from endpoint")
	var nameValues promNameValues
	resp, err := http.Get(prometheusURL + "/api/v1/label/__name__/values")
	checkErr(err)
	defer resp.Body.Close()
	log.Debugf("Received response code %v", strconv.Itoa(resp.StatusCode))

	respBody, err := ioutil.ReadAll(resp.Body)
	checkErr(err)
	err = json.Unmarshal([]byte(respBody), &nameValues)
	checkErr(err)

	sem := make(chan bool, concurrency)
	var mutex sync.Mutex

	for _, retention := range policy.Retentions {
		endTime := currentTime.Add(time.Duration(-retention.Seconds) * time.Second)

		for _, metric := range retention.Metrics {
			sem <- true
			go func(metr string, endT time.Time) {
				defer func() { <-sem }()

				log.Infof("Deleting data for %v before %v", metr, endT.Format(timeLayout))
				url := prometheusURL + "/api/v1/admin/tsdb/delete_series?match[]=" + metr + "&end=" + endT.Format(timeLayout)
				req, err := http.NewRequest(http.MethodPut, url, nil)
				checkErr(err)
				resp, err = client.Do(req)
				checkErr(err)
				defer resp.Body.Close()
				log.Debugf("Received response code %v for %v", strconv.Itoa(resp.StatusCode), metr)

				if policy.SetDefault {
					mutex.Lock()
					indexOf := -1
					for i, e := range nameValues.Data {
						if e == metr {
							indexOf = i
						}
					}
					if indexOf != -1 {
						log.Infof("Removing %v from default retention list", metr)
						nameValues.Data[indexOf] = nameValues.Data[len(nameValues.Data)-1]
						nameValues.Data[len(nameValues.Data)-1] = ""
						nameValues.Data = nameValues.Data[:len(nameValues.Data)-1]
					} else {
						log.Errorf("%v does not exist as a metric at this endpoint.", metr)
					}
					mutex.Unlock()
				} else {
					indexOf := -1
					for i, e := range nameValues.Data {
						if e == metr {
							indexOf = i
						}
					}
					if indexOf == -1 {
						log.Errorf("%v does not exist as a metric at this endpoint.", metr)
					}
				}
			}(metric, endTime)
		}

	}

	//Wait for all concurrencies to finish
	log.Debugf("Waiting for retentions to finish")
	for i := 0; i < cap(sem); i++ {
		sem <- true
	}

	//If a default is set, go through all the other metrics on the endpoint and set them to the default amount
	if policy.SetDefault {
		endTime := currentTime.Add(time.Duration(-policy.DefaultSeconds) * time.Second)
		log.Infof("Setting all other metrics to %v", endTime.Format(timeLayout))

		//Reset sem
		sem = make(chan bool, concurrency)

		for _, metric := range nameValues.Data {
			sem <- true
			go func(metr string) {
				defer func() { <-sem }()

				log.Infof("Deleting data for %v before %v", metr, endTime.Format(timeLayout))
				url := prometheusURL + "/api/v1/admin/tsdb/delete_series?match[]=" + metr + "&end=" + endTime.Format(timeLayout)
				req, err := http.NewRequest(http.MethodPut, url, nil)
				checkErr(err)
				resp, err = client.Do(req)
				checkErr(err)
				defer resp.Body.Close()
				log.Debugf("Received response code %v for %v", strconv.Itoa(resp.StatusCode), metr)
			}(metric)
		}

		log.Infof("Waiting for default metrics to finish")
		for i := 0; i < cap(sem); i++ {
			sem <- true
		}

		//Clean tombstones
		log.Infof("Cleaning tombstones")
		url := prometheusURL + "/api/v1/admin/tsdb/clean_tombstones"
		req, err := http.NewRequest(http.MethodPost, url, nil)
		checkErr(err)
		resp, err = client.Do(req)
		checkErr(err)
		defer resp.Body.Close()
		log.Debugf("Received response code %v for tombstones", strconv.Itoa(resp.StatusCode))
	}

	log.Infof("Finishing retention sweep on %v", prometheusURL)
}

func exitIf(condition bool, comment string) {
	if condition {
		log.Println(comment)
		os.Exit(1)
	}
}

func checkErr(err error) {
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
}

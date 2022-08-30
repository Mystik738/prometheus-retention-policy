package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	labels "github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/tsdb"
	log "github.com/sirupsen/logrus"
)

const (
	timeLayout  = "2006-01-02T15:04:05Z"
	concurrency = 8
	dataDir     = "/tmp/retention/"
)

type environment struct {
	Username string
	Password string
	Url      string
	Bucket   string
	Policy   policy
	Delta    int64
}

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
	sourceType, ok := os.LookupEnv("SOURCE_TYPE")
	//If sourcetype isn't set, default is http
	if !ok {
		sourceType = "api"
	}
	switch sourceType {
	case "api":
		runHttp()
	case "minio":
		runMinio()
	default:
		log.Panicf("%v is not a valid source type", sourceType)
	}
}

func runMinio() {
	env := loadEnv()
	log.SetLevel(log.DebugLevel)
	useSSL := true

	// Initialize minio client object.
	minioClient, err := minio.New(env.Url, &minio.Options{
		Creds:  credentials.NewStaticV4(env.Username, env.Password, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalln(err)
	}

	//Make a temp dir to store the blocks
	err = os.MkdirAll(dataDir, os.ModePerm)
	checkErr(err)

	//Look through our bucket and find objects within a delta of our retention periods
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dirs := minioClient.ListObjects(ctx, env.Bucket, minio.ListObjectsOptions{
		Prefix:    "/",
		Recursive: false,
	})

	blocks := make([]string, 0)
	for block := range dirs {
		log.Debugf("Object at %v", block.Key)

		//TODO: Check if there's a delete marker
		log.Debugf("Checking for deletion mark at %v", block.Key+"deletion-mark.json")

		_, err := minioClient.StatObject(ctx, env.Bucket, block.Key+"deletion-mark.json", minio.StatObjectOptions{})

		if err != nil {
			blocks = append(blocks, block.Key)
		} else {
			log.Debugf("Deletion mark exists, skipping block %v", block.Key)
		}
	}

	//For each block
	for _, block := range blocks {
		//Download the block
		log.Debugf("Deleting from block %v", block)
		blockObjects := minioClient.ListObjects(ctx, env.Bucket, minio.ListObjectsOptions{
			Prefix:    block,
			Recursive: true,
		})
		for bucketObject := range blockObjects {
			minioClient.FGetObject(ctx, env.Bucket, bucketObject.Key, dataDir+bucketObject.Key, minio.GetObjectOptions{})

		}

		//Initialize tsdb on the block
		//Delete series from block
		//Write new block
		runTsdb(block)

		//Upload new block
		//Delete local copies
		//Mark old block for deletion
	}
}

func runTsdb(blockName string) string {
	log.Debugf("Deleting from tsdb")
	db, err := tsdb.Open(dataDir, nil, nil, tsdb.DefaultOptions(), nil)
	checkErr(err)

	defer db.Close()
	env := loadEnv()

	block, err := tsdb.OpenBlock(nil, dataDir+blockName, nil)
	checkErr(err)

	meta := block.Meta()
	log.Infof("Block contains %v samples", meta.Stats.NumSamples)

	currentTime := time.Now()
	metricsPassed := make([]string, 0)

	for _, retention := range env.Policy.Retentions {
		log.Debugf("Checking metrics %v", retention.Metrics)
		endTime := currentTime.Add(time.Duration(-retention.Seconds) * time.Second)
		for i := range retention.Metrics {
			matchers, err := parser.ParseMetricSelector(retention.Metrics[i])
			checkErr(err)

			log.Infof("Deleting %v before %v", matchers, endTime)
			err = block.Delete(0, timestamp.FromTime(endTime), matchers...)
			checkErr(err)
		}
		metricsPassed = append(metricsPassed, retention.Metrics...)
	}

	if env.Policy.SetDefault {
		endTime := currentTime.Add(time.Duration(-env.Policy.DefaultSeconds) * time.Second)

		ir, err := block.Index()
		checkErr(err)

		matchers := make([]*labels.Matcher, 0)
		matchers = append(matchers, &labels.Matcher{
			Type:  labels.MatchNotEqual,
			Name:  "__name__",
			Value: "",
		})

		labelNames, err := ir.SortedLabelValues("__name__", matchers...)
		checkErr(err)

		sort.Strings(metricsPassed)
		checkErr(err)

		//We have two sorted slices and we want to remove elements in one slice from the other
		//This logic doesn't preserve sorting but works in O(n+m)
		j := 0
		rems := 0
		for _, rem := range metricsPassed {
			for strings.Compare(labelNames[j], rem) < 0 {
				j++
			}
			if strings.Compare(labelNames[j], rem) == 0 {
				labelNames[j] = labelNames[rems]
				rems++
			}
		}
		labelNames = labelNames[rems:]

		log.Infof("Setting all other metrics to %v", endTime.Format(timeLayout))
		for i := range labelNames {
			matchers, err := parser.ParseMetricSelector(labelNames[i])
			checkErr(err)
			log.Tracef("Deleting %v before %v", matchers, endTime)
			err = block.Delete(0, timestamp.FromTime(endTime), matchers...)
			checkErr(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmpt, err := tsdb.NewLeveledCompactor(ctx, nil, nil, []int64{1000000}, nil, nil)
	checkErr(err)
	newBlock, deleteParent, err := block.CleanTombstones(dataDir, cmpt)
	checkErr(err)

	log.Infof("New block at %v, %v", newBlock, deleteParent)

	meta = block.Meta()
	log.Infof("Block contains %v samples", meta.Stats.NumSamples)

	return newBlock.String()
}

func loadEnv() environment {
	endpoint, ok := os.LookupEnv("SOURCE_URL")
	exitIf(!ok, "No URL set, set environment variable SOURCE_URL")
	policyJSON, ok := os.LookupEnv("POLICY")
	exitIf(!ok, "No policy set, set environment variable POLICY")

	username, _ := os.LookupEnv("SOURCE_USERNAME")
	password, _ := os.LookupEnv("SOURCE_PASSWORD")
	bucket, _ := os.LookupEnv("SOURCE_BUCKET")

	strDelta, ok := os.LookupEnv("SEARCH_DELTA")
	if !ok {
		strDelta = "-1"
	}
	delta, err := strconv.Atoi(strDelta)
	checkErr(err)

	var policy policy
	err = json.Unmarshal([]byte(policyJSON), &policy)
	exitIf(err != nil, "Policy is not valid JSON policy")

	return environment{
		Url:      endpoint,
		Username: username,
		Password: password,
		Policy:   policy,
		Bucket:   bucket,
		Delta:    int64(delta),
	}
}

func runHttp() {
	env := loadEnv()

	currentTime := time.Now()
	log.Infof("Beginning retention sweep on %v at %v", env.Url, currentTime.Format(timeLayout))

	//Get all metrics from endpoint
	client := &http.Client{
		Timeout: 600 * time.Second,
	}

	log.Infof("Retrieving all metrics from endpoint")
	var nameValues promNameValues
	resp, err := http.Get(env.Url + "/api/v1/label/__name__/values")
	checkErr(err)
	defer resp.Body.Close()
	log.Debugf("Received response code %v", strconv.Itoa(resp.StatusCode))

	respBody, err := ioutil.ReadAll(resp.Body)
	checkErr(err)
	err = json.Unmarshal([]byte(respBody), &nameValues)
	checkErr(err)

	sem := make(chan bool, concurrency)
	var mutex sync.Mutex

	for _, retention := range env.Policy.Retentions {
		endTime := currentTime.Add(time.Duration(-retention.Seconds) * time.Second)

		for _, metric := range retention.Metrics {
			sem <- true
			go func(metr string, endT time.Time) {
				defer func() { <-sem }()

				log.Infof("Deleting data for %v before %v", metr, endT.Format(timeLayout))
				url := env.Url + "/api/v1/admin/tsdb/delete_series?match[]=" + metr + "&end=" + endT.Format(timeLayout)
				req, err := http.NewRequest(http.MethodPut, url, nil)
				checkErr(err)
				resp, err = client.Do(req)
				checkErr(err)
				defer resp.Body.Close()
				log.Debugf("Received response code %v for %v", strconv.Itoa(resp.StatusCode), metr)

				if env.Policy.SetDefault {
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
	if env.Policy.SetDefault {
		endTime := currentTime.Add(time.Duration(-env.Policy.DefaultSeconds) * time.Second)
		log.Infof("Setting all other metrics to %v", endTime.Format(timeLayout))

		//Reset sem
		sem = make(chan bool, concurrency)

		for _, metric := range nameValues.Data {
			sem <- true
			go func(metr string) {
				defer func() { <-sem }()

				log.Infof("Deleting data for %v before %v", metr, endTime.Format(timeLayout))
				url := env.Url + "/api/v1/admin/tsdb/delete_series?match[]=" + metr + "&end=" + endTime.Format(timeLayout)
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
		url := env.Url + "/api/v1/admin/tsdb/clean_tombstones"
		req, err := http.NewRequest(http.MethodPost, url, nil)
		checkErr(err)
		resp, err = client.Do(req)
		checkErr(err)
		defer resp.Body.Close()
		log.Debugf("Received response code %v for tombstones", strconv.Itoa(resp.StatusCode))
	}

	log.Infof("Finishing retention sweep on %v", env.Url)
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

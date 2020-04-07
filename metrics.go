package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func envString(key, def string) string {
	if env := os.Getenv(key); env != "" {
		return env
	}
	return def
}

var (
	defaultDuration, _ = time.ParseDuration(envString("SLEEP_DURATION", "5m"))
	patToken           = flag.String("pat", envString("PAT_TOKEN", ""), "pat token (env PAT_TOKEN)")
	addr               = flag.String("listen-address", envString("LISTEN_ADDRESS", ":8080"), "listen address. e.g. :8080 (env LISTEN_ADDRESS)")
	sleep              = flag.Duration("sleep", defaultDuration, "duration to sleep (default 5m) between gathering metrics from ADO (env SLEEP_DURATION)")
	buildsToWatch      = flag.String("builds", envString("BUILDS", ""), "build endpoints: org1/project1/definition1,org2/project2/definition2 (env BUILDS)")

	fetchCounts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ado_build_requests",
			Help: "ADO build metric request info",
		},
		[]string{"org", "project", "definition", "success"},
	)

	buildMetrics = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ado_build_metrics",
			Help: "ADO build metrics info",
		},
		[]string{"instance", "name", "scope", "date"},
	)
)

type buildResponse struct {
	Count int           `json:"count"`
	Value []buildMetric `json:"value"`
}

type buildMetric struct {
	Name     string `json:"name"`
	Scope    string `json:"scope"`
	IntValue int    `json:"intValue"`
	Date     string `json:"date"`
}

func logBuildMetrics(url string) (*buildResponse, error) {
	var netClient = &http.Client{
		Timeout: time.Second * 5,
	}
	httpReq, err := http.NewRequest("GET", url, nil)

	if err != nil {
		return nil, fmt.Errorf("error create request: %v", err)
	}

	httpReq.Header.Set("Accept", "application/json")

	resp, err := netClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error http.Do: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 203 {
		return nil, fmt.Errorf("non expected status code: %v", resp.StatusCode)
	}
	authData, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("error read response: %v", err)
	}

	var r buildResponse
	err = json.Unmarshal(authData, &r)
	if err != nil {
		return nil, fmt.Errorf("Error calling json.Unmarshal on the response: %v", err)
	}

	log.Printf("Fetched %v metrics\n", r.Count)
	return &r, nil
}

func collectMetrics() {
	for {
		builds := strings.Split(*buildsToWatch, ",")
		for _, build := range builds {
			parts := strings.Split(build, "/")
			if len(parts) == 3 {
				log.Printf("Fetching metrics for build: %v", build)
				url := fmt.Sprintf("https://user:%s@dev.azure.com/%s/%s/_apis/build/definitions/%s/metrics?api-version=5.1-preview.1", *patToken, parts[0], parts[1], parts[2])
				buildResponse, err := logBuildMetrics(url)
				success := "1"
				if err != nil {
					log.Printf("Error making request: %s\n", err)
					success = "0"
				}

				fetchCounts.WithLabelValues(parts[0], parts[1], parts[2], success).Inc()
				if buildResponse != nil {
					for _, bm := range buildResponse.Value {
						buildMetrics.WithLabelValues(build, bm.Name, bm.Scope, bm.Date).Set(float64(bm.IntValue))
					}
				}
			} else {
				log.Fatalf("Illegal part: %v\n", parts)
			}
		}

		log.Printf("Sleep for %v\n", *sleep)
		time.Sleep(*sleep)
	}
}

func main() {
	flag.Parse()
	if *patToken == "" {
		log.Fatal("PAT_TOKEN not set\n")
	}
	if *buildsToWatch == "" {
		log.Fatal("BUILDS not set\n")
	}

	prometheus.MustRegister(fetchCounts)
	prometheus.MustRegister(buildMetrics)
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())

	log.Printf("Listening on %s\n", *addr)
	go collectMetrics()
	http.Handle("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			// Opt into OpenMetrics to support exemplars.
			EnableOpenMetrics: true,
		},
	))
	log.Fatal(http.ListenAndServe(*addr, nil))
}

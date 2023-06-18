// Package middleware will measure metrics of different http handler types
// using a `metrics.Recorder`.
//
// The metrics measured are based on RED and/or Four golden signals.
package middleware

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/slok/go-http-metrics/metrics"
)

// Config is the configuration for the middleware factory.
type Config struct {
	// Recorder is the way the metrics will be recorder in the different backends.
	Recorder metrics.Recorder
	// Service is an optional identifier for the metrics, this can be useful if
	// a same service has multiple servers (e.g API, metrics and healthchecks).
	Service string
	// GroupedStatus will group the status label in the form of `\dxx`, for example,
	// 200, 201, and 203 will have the label `code="2xx"`. This impacts on the cardinality
	// of the metrics and also improves the performance of queries that are grouped by
	// status code because there are already aggregated in the metric.
	// By default will be false.
	GroupedStatus bool
	// DisableMeasureSize will disable the recording metrics about the response size,
	// by default measuring size is enabled (`DisableMeasureSize` is false).
	DisableMeasureSize bool
	// DisableMeasureInflight will disable the recording metrics about the inflight requests number,
	// by default measuring inflights is enabled (`DisableMeasureInflight` is false).
	DisableMeasureInflight bool
}

func (c *Config) defaults() {
	if c.Recorder == nil {
		c.Recorder = metrics.Dummy
	}
}

// Middleware is a service that knows how to measure an HTTP handler by wrapping
// another handler.
//
// Depending on the framework/library we want to measure, this can change a lot,
// to abstract the way how we measure on the different libraries, Middleware will
// recieve a `Reporter` that knows how to get the data the Middleware service needs
// to measure.
type Middleware struct {
	cfg Config
}

// New returns the a Middleware service.
func New(cfg Config) Middleware {
	cfg.defaults()

	m := Middleware{cfg: cfg}

	return m
}

func FixPath(urlPath string) string {
	if strings.Contains("urlPath", ".js") {
		return ""
	}
	if strings.Contains("urlPath", ".css") {
		return ""
	}
	if strings.Contains("urlPath", ".png") {
		return ""
	}
	if strings.Contains("urlPath", ".jpg") {
		return ""
	}
	if strings.Contains("urlPath", ".html") {
		return ""
	}
	if strings.Contains("urlPath", ".json") {
		return ""
	}
	tmpPaths := strings.Split(urlPath, "/")
	n := len(tmpPaths)
	if n <= 4 {
		return urlPath
	}
	pathResult := ""
	for i := 0; i < n; i++ {
		if regexp.MustCompile(`\d`).MatchString(tmpPaths[i]) && !strings.HasPrefix(tmpPaths[i], "v") {
			if i == n-1 {
				pathResult += "detail"
			}
		} else {
			pathResult += tmpPaths[i] + "/"
		}
	}

	pathResult = strings.TrimRight(pathResult, "/")
	return pathResult
}

// Measure abstracts the HTTP handler implementation by only requesting a reporter, this
// reporter will return the required data to be measured.
// it accepts a next function that will be called as the wrapped logic before and after
// measurement actions.
func (m Middleware) Measure(handlerID string, reporter Reporter, next func()) {
	ctx := reporter.Context()

	// If there isn't predefined handler ID we
	// set that ID as the URL path.
	hid := handlerID
	if handlerID == "" {
		hid = FixPath(reporter.URLPath())
	}

	// Measure inflights if required.
	if !m.cfg.DisableMeasureInflight {
		props := metrics.HTTPProperties{
			Service: m.cfg.Service,
			ID:      hid,
		}
		m.cfg.Recorder.AddInflightRequests(ctx, props, 1)
		defer m.cfg.Recorder.AddInflightRequests(ctx, props, -1)
	}

	// Start the timer and when finishing measure the duration.
	start := time.Now()
	defer func() {
		duration := time.Since(start)

		// If we need to group the status code, it uses the
		// first number of the status code because is the least
		// required identification way.
		var code string
		if m.cfg.GroupedStatus {
			code = fmt.Sprintf("%dxx", reporter.StatusCode()/100)
		} else {
			code = strconv.Itoa(reporter.StatusCode())
		}

		props := metrics.HTTPReqProperties{
			Service: m.cfg.Service,
			ID:      hid,
			Method:  reporter.Method(),
			Code:    code,
		}
		m.cfg.Recorder.ObserveHTTPRequestDuration(ctx, props, duration)

		// Measure size of response if required.
		if !m.cfg.DisableMeasureSize {
			m.cfg.Recorder.ObserveHTTPResponseSize(ctx, props, reporter.BytesWritten())
		}
	}()

	// Call the wrapped logic.
	next()
}

// Reporter knows how to report the data to the Middleware so it can measure the
// different framework/libraries.
type Reporter interface {
	Method() string
	Context() context.Context
	URLPath() string
	StatusCode() int
	BytesWritten() int64
}

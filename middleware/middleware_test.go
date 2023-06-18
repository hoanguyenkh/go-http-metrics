package middleware_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	mockmetrics "github.com/slok/go-http-metrics/internal/mocks/metrics"
	mockmiddleware "github.com/slok/go-http-metrics/internal/mocks/middleware"
	"github.com/slok/go-http-metrics/metrics"
	"github.com/slok/go-http-metrics/middleware"
)

func TestMiddlewareMeasure(t *testing.T) {
	tests := map[string]struct {
		handlerID string
		config    func() middleware.Config
		mock      func(mrec *mockmetrics.Recorder, mrep *mockmiddleware.Reporter)
	}{
		"Having default config with service, it should measure the metrics.": {
			handlerID: "test01",
			config: func() middleware.Config {
				return middleware.Config{
					Service: "svc1",
				}
			},
			mock: func(mrec *mockmetrics.Recorder, mrep *mockmiddleware.Reporter) {
				// Reporter mocks.
				mrep.On("Context").Once().Return(context.TODO())
				mrep.On("StatusCode").Once().Return(418)
				mrep.On("Method").Once().Return("PATCH")
				mrep.On("BytesWritten").Once().Return(int64(42))

				// Recorder mocks.
				expProps := metrics.HTTPProperties{Service: "svc1", ID: "test01"}
				expRepProps := metrics.HTTPReqProperties{Service: "svc1", ID: "test01", Method: "PATCH", Code: "418"}

				mrec.On("AddInflightRequests", mock.Anything, expProps, 1).Once()
				mrec.On("AddInflightRequests", mock.Anything, expProps, -1).Once()
				mrec.On("ObserveHTTPRequestDuration", mock.Anything, expRepProps, mock.Anything).Once()
				mrec.On("ObserveHTTPResponseSize", mock.Anything, expRepProps, int64(42)).Once()
			},
		},

		"Without having handler ID, it should measure the metrics using the request path.": {
			handlerID: "",
			config: func() middleware.Config {
				return middleware.Config{}
			},
			mock: func(mrec *mockmetrics.Recorder, mrep *mockmiddleware.Reporter) {
				// Reporter mocks.
				mrep.On("URLPath").Once().Return("/test/01")
				mrep.On("Context").Once().Return(context.TODO())
				mrep.On("StatusCode").Once().Return(418)
				mrep.On("Method").Once().Return("PATCH")
				mrep.On("BytesWritten").Once().Return(int64(42))

				// Recorder mocks.
				expRepProps := metrics.HTTPReqProperties{ID: "/test/01", Method: "PATCH", Code: "418"}

				mrec.On("AddInflightRequests", mock.Anything, mock.Anything, mock.Anything).Once()
				mrec.On("AddInflightRequests", mock.Anything, mock.Anything, mock.Anything).Once()
				mrec.On("ObserveHTTPRequestDuration", mock.Anything, expRepProps, mock.Anything).Once()
				mrec.On("ObserveHTTPResponseSize", mock.Anything, expRepProps, mock.Anything).Once()
			},
		},

		"Having grouped status code, it should measure the metrics using grouped status codes.": {
			handlerID: "test01",
			config: func() middleware.Config {
				return middleware.Config{
					GroupedStatus: true,
				}
			},
			mock: func(mrec *mockmetrics.Recorder, mrep *mockmiddleware.Reporter) {
				// Reporter mocks.
				mrep.On("Context").Once().Return(context.TODO())
				mrep.On("StatusCode").Once().Return(418)
				mrep.On("Method").Once().Return("PATCH")
				mrep.On("BytesWritten").Once().Return(int64(42))

				// Recorder mocks.
				expRepProps := metrics.HTTPReqProperties{ID: "test01", Method: "PATCH", Code: "4xx"}

				mrec.On("AddInflightRequests", mock.Anything, mock.Anything, mock.Anything).Once()
				mrec.On("AddInflightRequests", mock.Anything, mock.Anything, mock.Anything).Once()
				mrec.On("ObserveHTTPRequestDuration", mock.Anything, expRepProps, mock.Anything).Once()
				mrec.On("ObserveHTTPResponseSize", mock.Anything, expRepProps, mock.Anything).Once()
			},
		},

		"Disabling inflight requests measuring, it shouldn't measure inflight metrics.": {
			handlerID: "test01",
			config: func() middleware.Config {
				return middleware.Config{
					DisableMeasureInflight: true,
				}
			},
			mock: func(mrec *mockmetrics.Recorder, mrep *mockmiddleware.Reporter) {
				// Reporter mocks.
				mrep.On("Context").Once().Return(context.TODO())
				mrep.On("StatusCode").Once().Return(418)
				mrep.On("Method").Once().Return("PATCH")
				mrep.On("BytesWritten").Once().Return(int64(42))

				// Recorder mocks.
				expRepProps := metrics.HTTPReqProperties{ID: "test01", Method: "PATCH", Code: "418"}

				mrec.On("ObserveHTTPRequestDuration", mock.Anything, expRepProps, mock.Anything).Once()
				mrec.On("ObserveHTTPResponseSize", mock.Anything, expRepProps, mock.Anything).Once()
			},
		},

		"Disabling size measuring, it shouldn't measure size metrics.": {
			handlerID: "test01",
			config: func() middleware.Config {
				return middleware.Config{
					DisableMeasureSize: true,
				}
			},
			mock: func(mrec *mockmetrics.Recorder, mrep *mockmiddleware.Reporter) {
				// Reporter mocks.
				mrep.On("Context").Once().Return(context.TODO())
				mrep.On("StatusCode").Once().Return(418)
				mrep.On("Method").Once().Return("PATCH")

				// Recorder mocks.
				expRepProps := metrics.HTTPReqProperties{ID: "test01", Method: "PATCH", Code: "418"}

				mrec.On("AddInflightRequests", mock.Anything, mock.Anything, mock.Anything).Once()
				mrec.On("AddInflightRequests", mock.Anything, mock.Anything, mock.Anything).Once()
				mrec.On("ObserveHTTPRequestDuration", mock.Anything, expRepProps, mock.Anything).Once()
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)

			// Mocks.
			mrec := &mockmetrics.Recorder{}
			mrep := &mockmiddleware.Reporter{}
			test.mock(mrec, mrep)

			// Execute.
			config := test.config()
			config.Recorder = mrec // Set mocked recorder.
			mdlw := middleware.New(config)

			calledNext := false
			mdlw.Measure(test.handlerID, mrep, func() { calledNext = true })

			// Check.
			mrec.AssertExpectations(t)
			mrep.AssertExpectations(t)
			assert.True(calledNext)
		})
	}
}

func Test_fixPath(t *testing.T) {
	res := middleware.FixPath("/api/v1/wallet/txs-history/0xe25fd558516cd5bf20b5d3dcb598dac566a61d1c")
	assert.Equal(t, "/api/v1/wallet/txs-history/detail", res)

	res = middleware.FixPath("/api/v1/brands")
	assert.Equal(t, "/api/v1/brands", res)

	res = middleware.FixPath("/api/v1/brands/123")
	assert.Equal(t, "/api/v1/brands/detail", res)

	res = middleware.FixPath("/api/v1/brands/cashback")
	assert.Equal(t, "/api/v1/brands/cashback", res)

	res = middleware.FixPath("/api/v1/brands/detail/210")
	assert.Equal(t, "/api/v1/brands/detail/detail", res)

	res = middleware.FixPath("/api/v1/coupons/2220")
	assert.Equal(t, "/api/v1/coupons/detail", res)

	res = middleware.FixPath("/api/v1/orders/247643/claim")
	assert.Equal(t, "/api/v1/orders/claim", res)

	res = middleware.FixPath("/api/v1/orders/used")
	assert.Equal(t, "/api/v1/orders/used", res)

	res = middleware.FixPath("/api/v1/brands/cashback")
	assert.Equal(t, "/api/v1/brands/cashback", res)

	res = middleware.FixPath("/api/v1/brands/cashback/456")
	assert.Equal(t, "/api/v1/brands/cashback/detail", res)

	res = middleware.FixPath("/api/v1/user-brands/favorite")
	assert.Equal(t, "/api/v1/user-brands/favorite", res)

	res = middleware.FixPath("/api/v1/user-brands/favorite/123")
	assert.Equal(t, "/api/v1/user-brands/favorite/detail", res)

	res = middleware.FixPath("/api/v1/user-coupons/overview/672")
	assert.Equal(t, "/api/v1/user-coupons/overview/detail", res)
}

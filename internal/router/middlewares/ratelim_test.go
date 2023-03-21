package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLimit1Addr(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		callRPS  int
		limitRPS int
	}

	tests := []testCase{
		{name: "success", callRPS: 100, limitRPS: 500},
		{name: "block-me", callRPS: 1000, limitRPS: 500},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				cfg := RateLimiterConfig{
					Default: RateLimiterRouteConfig{
						MaxRPI:   uint64(tc.limitRPS),
						Interval: time.Second,
					},
				}
				rlcm, err := RateLimitController(cfg)
				require.NoError(t, err)
				rlc := rlcm(dummyHandler{})

				ctx := context.WithValue(context.Background(), ContextKeyAddress, "0xdeadbeef")
				r, err := http.NewRequestWithContext(ctx, "", "", nil)
				require.NoError(t, err)

				res := httptest.NewRecorder()

				// Verify that after some seconds making requests with the configured
				// callRPS with the limitRPS, we are getting the expected output:
				// - If callRPS < limitRPS, we never get a 429.
				// - If callRPS > limitRPS, we eventually should see a 429.
				assertFunc := require.Eventually
				if tc.callRPS < tc.limitRPS {
					assertFunc = require.Never
				}
				assertFunc(t, func() bool {
					rlc.ServeHTTP(res, r)
					return res.Code == 429
				}, time.Second*5, time.Second/time.Duration(tc.callRPS))
			}
		}(tc))
	}
}

func TestLimit1IP(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		callRPS      int
		limitRPS     int
		forwardedFor bool
	}

	tests := []testCase{
		{name: "forwarded-success", callRPS: 100, limitRPS: 500, forwardedFor: true},
		{name: "forwarded-block-me", callRPS: 1000, limitRPS: 500, forwardedFor: true},

		{name: "success", callRPS: 100, limitRPS: 500, forwardedFor: false},
		{name: "block-me", callRPS: 1000, limitRPS: 500, forwardedFor: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				cfg := RateLimiterConfig{
					Default: RateLimiterRouteConfig{
						MaxRPI:   uint64(tc.limitRPS),
						Interval: time.Second,
					},
				}
				rlcm, err := RateLimitController(cfg)
				require.NoError(t, err)
				rlc := rlcm(dummyHandler{})

				ctx := context.Background()
				r, err := http.NewRequestWithContext(ctx, "", "", nil)
				require.NoError(t, err)

				if tc.forwardedFor {
					r.Header.Set("X-Forwarded-For", uuid.NewString())
				} else {
					r.RemoteAddr = uuid.NewString() + ":1234"
				}

				res := httptest.NewRecorder()

				// Verify that after some seconds making requests with the configured
				// callRPS with the limitRPS, we are getting the expected output:
				// - If callRPS < limitRPS, we never get a 429.
				// - If callRPS > limitRPS, we eventually should see a 429.
				assertFunc := require.Eventually
				if tc.callRPS < tc.limitRPS {
					assertFunc = require.Never
				}
				assertFunc(t, func() bool {
					rlc.ServeHTTP(res, r)
					return res.Code == 429
				}, time.Second*5, time.Second/time.Duration(tc.callRPS))
			}
		}(tc))
	}
}

func TestRateLim10Addresses(t *testing.T) {
	t.Parallel()

	// Only allow 150 req per second *per address*.
	cfg := RateLimiterConfig{
		Default: RateLimiterRouteConfig{
			MaxRPI:   150,
			Interval: time.Second,
		},
	}
	rlcm, err := RateLimitController(cfg)
	require.NoError(t, err)
	rlc := rlcm(dummyHandler{})

	// Do 1000 requests as fast as we can with *different addresses*, and see that
	// we never get a 429 status response. The request per second being done is
	// clearly more than 10 per second, but from different addresses which should be fine.
	for i := 0; i < 1000; i++ {
		// TODO: I'm not sure how to handle this test now that the JSON-RPC API is removed
		//		 and the SIWE token is never being included. -JW
		ctx := context.WithValue(context.Background(), ContextKeyAddress, strconv.Itoa(i%10))
		r, err := http.NewRequestWithContext(ctx, "", "", nil)
		require.NoError(t, err)

		res := httptest.NewRecorder()

		rlc.ServeHTTP(res, r)
		require.Equal(t, 200, res.Code)
	}
}

func TestRateLim10IPs(t *testing.T) {
	t.Parallel()

	// Only allow 10 req per second *per address*.
	cfg := RateLimiterConfig{
		Default: RateLimiterRouteConfig{
			MaxRPI:   100,
			Interval: time.Second,
		},
	}
	rlcm, err := RateLimitController(cfg)
	require.NoError(t, err)
	rlc := rlcm(dummyHandler{})

	// Do 1000 requests as fast as we can with *different IPs*, and see that
	// we never get a 429 status response. The request per second being done is
	// clearly more than 10 per second, but from different addresses which should be fine.
	for i := 0; i < 1000; i++ {
		ctx := context.WithValue(context.Background(), ContextKeyAddress, strconv.Itoa(i))
		r, err := http.NewRequestWithContext(ctx, "", "", nil)
		require.NoError(t, err)
		r.Header.Set("X-Forwarded-For", uuid.NewString())

		res := httptest.NewRecorder()

		rlc.ServeHTTP(res, r)
		require.Equal(t, 200, res.Code)
	}
}

type dummyHandler struct{}

func (dh dummyHandler) ServeHTTP(_ http.ResponseWriter, _ *http.Request) {
}

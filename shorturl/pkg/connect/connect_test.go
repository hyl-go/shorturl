package connect

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// 单测使用 mock Transport，不发起真实 TCP；生产环境须使用 ssrf.NewUserURLHTTPClient。
func TestGet_mockTransport(t *testing.T) {
	ctx := context.Background()
	convey.Convey("2xx 视为可访问", t, func() {
		cli := &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    r,
				}, nil
			}),
		}
		convey.So(Get(ctx, cli, "https://example.com/ok"), convey.ShouldBeTrue)
	})

	convey.Convey("204 No Content 仍为成功（2xx）", t, func() {
		cli := &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    r,
				}, nil
			}),
		}
		convey.So(Get(ctx, cli, "https://example.com/nocontent"), convey.ShouldBeTrue)
	})

	convey.Convey("404 视为不可访问", t, func() {
		cli := &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("")),
					Request:    r,
				}, nil
			}),
		}
		convey.So(Get(ctx, cli, "https://example.com/missing"), convey.ShouldBeFalse)
	})

	convey.Convey("非法 URL", t, func() {
		cli := &http.Client{
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return nil, nil
			}),
		}
		convey.So(Get(ctx, cli, "not-a-url"), convey.ShouldBeFalse)
	})

	convey.Convey("nil client 为 false", t, func() {
		convey.So(Get(ctx, nil, "https://example.com/"), convey.ShouldBeFalse)
	})
}

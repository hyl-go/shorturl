package connect

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestGet_httptest(t *testing.T) {
	convey.Convey("2xx 视为可访问", t, func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		convey.So(Get(srv.URL), convey.ShouldBeTrue)
	})

	convey.Convey("204 No Content 仍为成功（2xx）", t, func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()
		convey.So(Get(srv.URL), convey.ShouldBeTrue)
	})

	convey.Convey("404 视为不可访问", t, func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()
		convey.So(Get(srv.URL), convey.ShouldBeFalse)
	})

	convey.Convey("非法 URL", t, func() {
		convey.So(Get("not-a-url"), convey.ShouldBeFalse)
	})
}

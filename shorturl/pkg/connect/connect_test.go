package connect

import (
	"github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestGet(t *testing.T) {
	convey.Convey("基础用例", t, func() {
		url := "https://www.liwenzhou.com/posts/Go/golang-menu/"
		got := Get(url)
		// 断言
		convey.So(got, convey.ShouldEqual, true)
	})

	convey.Convey("url请求不通", t, func() {
		url := "https://www.liwenzhou.com"
		got := Get(url)
		// 断言
		convey.So(got, convey.ShouldBeFalse)
	})
}

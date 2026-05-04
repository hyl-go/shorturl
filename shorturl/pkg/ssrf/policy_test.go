package ssrf

import (
	"net"
	"net/url"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestPolicy_ValidateRequestURL(t *testing.T) {
	p := Policy{}
	convey.Convey("仅 http(s)", t, func() {
		u, _ := url.Parse("gopher://evil/")
		convey.So(p.ValidateRequestURL(u), convey.ShouldNotBeNil)
		u2, _ := url.Parse("https://example.com/")
		convey.So(p.ValidateRequestURL(u2), convey.ShouldBeNil)
	})
	convey.Convey("禁止 userinfo", t, func() {
		u, _ := url.Parse("http://user@example.com/")
		convey.So(p.ValidateRequestURL(u), convey.ShouldNotBeNil)
	})
	convey.Convey("字面量 127.0.0.1", t, func() {
		u, _ := url.Parse("http://127.0.0.1/")
		convey.So(p.ValidateRequestURL(u), convey.ShouldNotBeNil)
	})
	convey.Convey("AllowPrivateTargets 放行字面量回环", t, func() {
		pr := Policy{AllowPrivateTargets: true}
		u, _ := url.Parse("http://127.0.0.1/")
		convey.So(pr.ValidateRequestURL(u), convey.ShouldBeNil)
	})
	convey.Convey("OnlyStdPorts", t, func() {
		p2 := Policy{OnlyStdPorts: true}
		u, _ := url.Parse("http://example.com:8080/")
		convey.So(p2.ValidateRequestURL(u), convey.ShouldNotBeNil)
		u2, _ := url.Parse("https://example.com/")
		convey.So(p2.ValidateRequestURL(u2), convey.ShouldBeNil)
	})
}

func TestIsBlockedIP(t *testing.T) {
	convey.Convey("私网与环回", t, func() {
		convey.So(isBlockedIP(net.ParseIP("10.0.0.1")), convey.ShouldBeTrue)
		convey.So(isBlockedIP(net.ParseIP("192.168.1.1")), convey.ShouldBeTrue)
		convey.So(isBlockedIP(net.ParseIP("127.0.0.1")), convey.ShouldBeTrue)
		convey.So(isBlockedIP(net.ParseIP("169.254.1.1")), convey.ShouldBeTrue)
		convey.So(isBlockedIP(net.ParseIP("100.64.0.1")), convey.ShouldBeTrue)
		convey.So(isBlockedIP(net.ParseIP("8.8.8.8")), convey.ShouldBeFalse)
	})
}

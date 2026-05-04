// Package ssrf 对用户可控 URL 的出站请求做架构层约束，降低 SSRF（含 DNS 重绑定、重定向跳转）风险。
package ssrf

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Policy 出站请求策略（由配置映射而来）。
type Policy struct {
	// OnlyStdPorts 为 true 时仅允许 http:80、https:443（不含显式端口或标准端口）。
	OnlyStdPorts bool
	// AllowPrivateTargets 为 true 时不拦截环回/私网/链路本地等解析结果（仅用于本地或受信内网演示；公网生产必须为 false）。
	AllowPrivateTargets bool
}

func (p Policy) ipBlocked(ip net.IP) bool {
	if p.AllowPrivateTargets {
		return false
	}
	return isBlockedIP(ip)
}

func (p Policy) addrsBlocked(ips []net.IPAddr) bool {
	if p.AllowPrivateTargets {
		return false
	}
	for _, a := range ips {
		if isBlockedIP(a.IP) {
			return true
		}
	}
	return false
}

// ValidateRequestURL 校验即将请求的 URL（含重定向后的目标）。
func (p Policy) ValidateRequestURL(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("ssrf: empty url")
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("ssrf: 仅允许 http/https 协议")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("ssrf: 缺少主机名")
	}
	// 禁止在 URL 中携带凭据，避免与内网认证结合扩大危害面
	if u.User != nil {
		if _, ok := u.User.Password(); ok {
			return fmt.Errorf("ssrf: 不允许 URL 内嵌密码")
		}
		if u.User.Username() != "" {
			return fmt.Errorf("ssrf: 不允许 URL 内嵌用户名")
		}
	}
	port := effectivePort(u, scheme)
	if p.OnlyStdPorts {
		if !((scheme == "http" && port == "80") || (scheme == "https" && port == "443")) {
			return fmt.Errorf("ssrf: 当前策略仅允许标准端口 80/443")
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		if p.ipBlocked(ip) {
			return fmt.Errorf("ssrf: 禁止访问该 IP 地址")
		}
		return nil
	}
	// 域名在连接阶段由 DialContext 解析后再校验解析结果
	return nil
}

func effectivePort(u *url.URL, scheme string) string {
	port := u.Port()
	if port != "" {
		return port
	}
	if scheme == "https" {
		return "443"
	}
	return "80"
}

// isBlockedIP 拦截私网、环回、链路本地、组播及常见「文档/保留」段（含 100.64.0.0/10 CGNAT）。
func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 100.64.0.0/10 运营商级 NAT，Go IsPrivate 未覆盖
		if ip4[0] == 100 && ip4[1]&0xc0 == 64 {
			return true
		}
		// 192.0.0.0/24 IETF Protocol Assignments
		if ip4[0] == 192 && ip4[1] == 0 && ip4[2] == 0 {
			return true
		}
	}
	return false
}

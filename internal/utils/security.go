package utils

import (
	"net"
	"net/http"
	"strings"
)

// ExtractBearerToken 从 Authorization 头部解析 Bearer token。
func ExtractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	header = strings.TrimSpace(header)
	if len(header) < 7 {
		return ""
	}
	if strings.ToLower(header[:6]) != "bearer" {
		return ""
	}
	return strings.TrimSpace(header[6:])
}

// ResolveRequestToken 综合 Header 与 Query 解析访问令牌。
func ResolveRequestToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	token := ExtractBearerToken(r.Header.Get("Authorization"))
	if token != "" {
		return token
	}
	if queryToken := r.URL.Query().Get("access_token"); queryToken != "" {
		return queryToken
	}
	return ""
}

// ClientKey 根据请求推导限流 key，优先使用 token 其次使用 IP。
func ClientKey(r *http.Request, token string) string {
	if token != "" {
		return token
	}
	if r == nil {
		return "anonymous"
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "anonymous"
	}
	return host
}

package cherryConnector

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type WrapperWSConn struct {
	r *http.Request
	*WSConn
}

func NewWrapperWSConn(r *http.Request, wsConn *WSConn) *WrapperWSConn {
	return &WrapperWSConn{
		r:      r,
		WSConn: wsConn,
	}
}

func (c *WrapperWSConn) RemoteAddr() net.Addr {
	ipp, _ := netip.ParseAddrPort(c.r.RemoteAddr)
	defaultIp := ipp.Addr().String()

	ip := strings.TrimSpace(strings.Split(c.r.Header.Get("X-Original-Forwarded-For"), ",")[0])
	if ip != "" {
		goto Result
	}

	ip = strings.TrimSpace(strings.Split(c.r.Header.Get("X-Forwarded-For"), ",")[0])
	if ip != "" {
		goto Result
	}

	ip = strings.TrimSpace(c.r.Header.Get("X-Real-Ip"))
	if ip != "" {
		goto Result
	}

	if ip == "" {
		ip = defaultIp
	}

Result:
	addr, _ := netip.ParseAddr(ip)
	return net.TCPAddrFromAddrPort(netip.AddrPortFrom(addr, 0))
}

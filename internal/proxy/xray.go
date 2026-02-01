package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"proxyctx/internal/config"

	xnet "github.com/xtls/xray-core/common/net"
	xcore "github.com/xtls/xray-core/core"
	_ "github.com/xtls/xray-core/main/distro/all"
	_ "github.com/xtls/xray-core/main/json"
)

type Instance struct {
	Core       *xcore.Instance
	ListenAddr string
	ListenPort int
	ProxyURL   string
}

func Start(p config.Proxy, listenAddr string, listenPort int) (*Instance, error) {
	if listenAddr == "" {
		listenAddr = "127.0.0.1"
	}
	if listenPort == 0 {
		port, err := freePort(listenAddr)
		if err != nil {
			return nil, err
		}
		listenPort = port
	}

	cfgBytes, err := buildConfig(p, listenAddr, listenPort)
	if err != nil {
		return nil, err
	}
	inst, err := xcore.StartInstance("json", cfgBytes)
	if err != nil {
		return nil, err
	}

	return &Instance{
		Core:       inst,
		ListenAddr: listenAddr,
		ListenPort: listenPort,
		ProxyURL:   fmt.Sprintf("socks5://%s:%d", listenAddr, listenPort),
	}, nil
}

func (i *Instance) Close() {
	if i == nil || i.Core == nil {
		return
	}
	_ = i.Core.Close()
}

func MeasureLatency(ctx context.Context, proxy config.Proxy, target string, timeout time.Duration) (time.Duration, error) {
	if timeout == 0 {
		timeout = 8 * time.Second
	}
	inst, err := Start(proxy, "127.0.0.1", 0)
	if err != nil {
		return 0, err
	}
	defer inst.Close()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	if err := fetchOnce(ctx, inst.Core, target); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

func fetchOnce(ctx context.Context, inst *xcore.Instance, target string) error {
	u, err := url.Parse(target)
	if err != nil {
		return err
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	if u.Host == "" {
		return errors.New("benchmark url missing host")
	}
	port := u.Port()
	if port == "" {
		if strings.EqualFold(u.Scheme, "https") {
			port = "443"
		} else {
			port = "80"
		}
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, portStr, splitErr := net.SplitHostPort(addr)
			if splitErr != nil {
				host = u.Hostname()
				portStr = port
			}
			portNum, err := strconv.Atoi(portStr)
			if err != nil {
				return nil, err
			}
			dest := xnet.TCPDestination(xnet.ParseAddress(host), xnet.Port(portNum))
			return xcore.Dial(ctx, inst, dest)
		},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.Body.Close()
}

func freePort(listenAddr string) (int, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort(listenAddr, "0"))
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

func buildConfig(p config.Proxy, listenAddr string, listenPort int) ([]byte, error) {
	outbound, err := buildOutbound(p)
	if err != nil {
		return nil, err
	}

	cfg := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "none",
			"access":   "none",
			"error":    "none",
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"listen":   listenAddr,
				"port":     listenPort,
				"protocol": "socks",
				"settings": map[string]interface{}{
					"udp": true,
				},
			},
		},
		"outbounds": []interface{}{outbound},
	}

	return json.Marshal(cfg)
}

func buildOutbound(p config.Proxy) (map[string]interface{}, error) {
	protocol := strings.ToLower(p.Type)
	if protocol == "" {
		return nil, errors.New("proxy type is required")
	}
	if p.Server == "" || p.Port <= 0 {
		return nil, errors.New("proxy server and port are required")
	}
	if p.UUID == "" {
		return nil, errors.New("proxy uuid is required")
	}

	outbound := map[string]interface{}{
		"protocol": protocol,
	}

	settings := map[string]interface{}{}
	switch protocol {
	case "vmess":
		security := p.Cipher
		if security == "" {
			security = "auto"
		}
		settings["vnext"] = []interface{}{
			map[string]interface{}{
				"address": p.Server,
				"port":    p.Port,
				"users": []interface{}{
					map[string]interface{}{
						"id":       p.UUID,
						"security": security,
					},
				},
			},
		}
	case "vless":
		user := map[string]interface{}{
			"id":         p.UUID,
			"encryption": "none",
		}
		if p.Flow != "" {
			user["flow"] = p.Flow
		}
		settings["vnext"] = []interface{}{
			map[string]interface{}{
				"address": p.Server,
				"port":    p.Port,
				"users":   []interface{}{user},
			},
		}
	default:
		return nil, fmt.Errorf("unsupported proxy type: %s", protocol)
	}

	outbound["settings"] = settings

	streamSettings := map[string]interface{}{}
	network := strings.ToLower(p.Network)
	if network == "" && p.WSOpts != nil {
		network = "ws"
	}
	if network != "" {
		streamSettings["network"] = network
	}
	if p.TLS {
		streamSettings["security"] = "tls"
		tlsSettings := map[string]interface{}{
			"allowInsecure": p.SkipCertVerify,
		}
		if p.ServerName != "" {
			tlsSettings["serverName"] = p.ServerName
		}
		if p.ClientFingerprint != "" {
			tlsSettings["fingerprint"] = p.ClientFingerprint
		}
		streamSettings["tlsSettings"] = tlsSettings
	}
	if network == "ws" || p.WSOpts != nil {
		wsSettings := map[string]interface{}{}
		if p.WSOpts != nil {
			if p.WSOpts.Path != "" {
				wsSettings["path"] = p.WSOpts.Path
			}
			if len(p.WSOpts.Headers) > 0 {
				wsSettings["headers"] = p.WSOpts.Headers
			}
		}
		if len(wsSettings) > 0 {
			streamSettings["wsSettings"] = wsSettings
		}
	}
	if p.TFO {
		streamSettings["sockopt"] = map[string]interface{}{
			"tcpFastOpen": true,
		}
	}
	if len(streamSettings) > 0 {
		outbound["streamSettings"] = streamSettings
	}

	return outbound, nil
}

package selector

import (
	"context"
	"errors"
	"log"
	"path"
	"strings"
	"time"

	"proxyctx/internal/config"
)

type Tester func(ctx context.Context, proxy config.Proxy) (time.Duration, error)

func SelectProxy(ctx context.Context, cfg *config.Config, tester Tester, logger *log.Logger) (config.Proxy, error) {
	candidates, err := matchProxies(cfg.Proxies, cfg.Choise)
	if err != nil {
		return config.Proxy{}, err
	}
	if len(candidates) == 1 || tester == nil {
		return candidates[0], nil
	}

	var (
		best      config.Proxy
		bestFound bool
		bestRTT   time.Duration
	)

	for _, candidate := range candidates {
		start := time.Now()
		rtt, err := tester(ctx, candidate)
		if err != nil {
			if logger != nil {
				logger.Printf("speedtest failed: %s (%v)", candidate.Name, err)
			}
			continue
		}
		if rtt <= 0 {
			rtt = time.Since(start)
		}
		if logger != nil {
			logger.Printf("speedtest ok: %s (%s)", candidate.Name, rtt.Round(time.Millisecond))
		}
		if !bestFound || rtt < bestRTT {
			best = candidate
			bestRTT = rtt
			bestFound = true
		}
	}

	if !bestFound {
		return config.Proxy{}, errors.New("all candidates failed speed test")
	}
	return best, nil
}

func matchProxies(all []config.Proxy, choice string) ([]config.Proxy, error) {
	if choice == "" {
		return all, nil
	}
	useGlob := strings.ContainsAny(choice, "*?[")
	var matched []config.Proxy
	for _, proxy := range all {
		if useGlob {
			ok, err := path.Match(choice, proxy.Name)
			if err != nil {
				return nil, err
			}
			if ok {
				matched = append(matched, proxy)
			}
			continue
		}
		if proxy.Name == choice {
			matched = append(matched, proxy)
		}
	}
	if len(matched) == 0 {
		return nil, errors.New("no proxy matched choise")
	}
	return matched, nil
}

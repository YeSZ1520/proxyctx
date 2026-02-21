package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"proxyctx/internal/config"
	"proxyctx/internal/proxy"
	"proxyctx/internal/runner"
	"proxyctx/internal/selector"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	var configPath string

	fs := flag.NewFlagSet("proxyctx", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&configPath, "config", "", "path to config file")
	fs.Usage = func() {
		bin := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stderr, "Usage: %s [--config path] <command> [args...]\n", bin)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	cmdArgs := fs.Args()
	if len(cmdArgs) == 0 {
		fs.Usage()
		return 2
	}

	if configPath == "" {
		path, err := config.FindDefaultConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "proxyctx: %v\n", err)
			return 1
		}
		configPath = path
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "proxyctx: failed to load config %q: %v\n", configPath, err)
		return 1
	}

	logger := log.New(os.Stderr, "proxyctx: ", 0)
	logger.Printf("using config: %s", configPath)

	target := cfg.BenchmarkTarget()
	choice := cfg.Choise
	if choice == "" {
		choice = "*"
	}
	logger.Printf("selecting proxy (choice=%q, benchmark=%s)", choice, target)

	selected, err := selector.SelectProxy(
		context.Background(),
		cfg,
		func(ctx context.Context, p config.Proxy) (time.Duration, error) {
			return proxy.MeasureLatency(ctx, p, target, 8*time.Second)
		},
		logger,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "proxyctx: failed to select proxy: %v\n", err)
		return 1
	}
	logger.Printf(
		"selected proxy: %s (%s %s:%d)",
		selected.Name,
		strings.ToLower(selected.Type),
		selected.Server,
		selected.Port,
	)

	inst, err := proxy.Start(selected, "127.0.0.1", 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "proxyctx: failed to start local proxy: %v\n", err)
		return 1
	}
	defer inst.Close()
	logger.Printf("local proxy ready: %s", inst.ProxyURL)

	env := map[string]string{
		"http_proxy":  inst.ProxyURL,
		"https_proxy": inst.ProxyURL,
		"all_proxy":   inst.ProxyURL,
		"HTTP_PROXY":  inst.ProxyURL,
		"HTTPS_PROXY": inst.ProxyURL,
		"ALL_PROXY":   inst.ProxyURL,
	}

	logger.Printf("running command: %s", strings.Join(cmdArgs, " "))
	code, err := runner.Run(cmdArgs, env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "proxyctx: failed to run command: %v\n", err)
	}
	return code
}

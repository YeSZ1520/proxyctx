# proxyctx

`proxyctx` 是一个命令行代理包装器：启动本地代理，设置代理环境变量，然后执行命令。

## 用法

```bash
proxyctx curl https://api.ipify.org
```

指定配置文件：

```bash
proxyctx --config ./config.yaml curl https://api.ipify.org
```

## 配置文件搜索顺序

未传 `--config` 时，会按以下顺序查找：

1. `./.config/proxyctx/config.yaml`
3. `~/.config/proxyctx/config.yaml`


## 配置格式

`proxyctx` 兼容 Clash 的配置文件格式（使用 `proxies` 列表），并额外支持 `choise`、`benchmark` 字段。

- proxies：代理列表
- choise：选择的节点
- benchmark：测试的基准网站

必填字段：`proxies`。  

`choise` 支持 `*` 通配符且可选。若未填写或匹配多个节点，会测速并选择最快的。

```yaml
proxies:
  - name: "my_node"
    type: vmess
    server: www.example.com
    port: 443
    uuid: 11111111-2222-3333-4444-555555555555
    cipher: auto
    tls: true
    skip-cert-verify: true
    network: ws
    ws-opts:
      path: /ws
      headers:
        Host: www.example.com

choise: "my_node"
benchmark: "https://www.google.com/generate_204"
```

支持的代理字段（Clash-like 子集）：

- `name`, `type` (`vmess` / `vless`), `server`, `port`, `uuid`
- `cipher` (vmess), `flow` (vless)
- `tls`, `skip-cert-verify`, `servername`, `client-fingerprint`
- `network`（如 `ws`）, `ws-opts.path`, `ws-opts.headers`
- `tfo`

## 行为说明

- 启动内嵌 Xray-core 和本地 SOCKS5 监听。
- 设置 `http_proxy`, `https_proxy`, `all_proxy`（含大写变体）。
- 运行目标命令。
- 目标命令启动后不再输出任何日志。

## 构建

```bash
go build ./cmd/proxyctx
```

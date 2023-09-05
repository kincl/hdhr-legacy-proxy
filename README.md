# hdhr-legacy-proxy

This is a proxy that will make a HDHR legacy (HDHR Dual, HDHR 3) act like a newer version
in order to work with Plex Media Center

![diagram](design-docs/hdhr-legacy-proxy.jpg)

## Use

```
docker run -it -p 8000:8000/tcp -p 6000:6000/udp ghcr.io/kincl/hdhr-legacy-proxy:latest
```

Environment Options

| Option | Description | Default |
| ------ | ----------- | ------- |
| HDHR_LEGACY_PROXY_HOST | IP of the host running the proxy | (none) |
| HDHR_LEGACY_PROXY_PORT | Port the proxy listens on for connections from Plex | 8000 |
| HDHR_LEGACY_PROXY_TUNER_PORT | UDP port the proxy listens on for connections from the HDHR tuner | 6000 |

### Channel Scan

Access `http://${HDHR_LEGACY_PROXY_HOST}:${HDHR_LEGACY_PROXY_PORT}/scan` to do a channel scan and put the results in memory

### Plex

Manually specify the tuner in the settings as `${HDHR_LEGACY_PROXY_HOST}:${HDHR_LEGACY_PROXY_PORT}`

## TODO

- Default to using own IP for proxy host variable
- Option for manually specifying device and not doing autodiscovery
- Document how to use with Docker and Kubernetes
- Implement channel scan results storage

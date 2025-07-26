# cluster-app

a small program to run on your Linux desktop or WSL to observe the status from your very important services.
metrics are fetched from your existing Prometheus backend and displayed on the cluster-app window.

configure your relevant metrics in [metrics.json](metrics.json), download the binary from the [Release page](https://github.com/eumel8/cluster-app/releases) and start the program in the same directory where the metric.json exists.

point `PROMETHEUS_URL` env to your Prometheus backend, i.e. `http://prometheus.example.com:9090`

start the program and enjoy

## Vulkan driver in WSL

```
sudo apt install vulkan-tools
```

## Bitwarden version

In this version the programm expect an item on a Bitwarden service containing username/password for HTTP Basic Auth on
Prometheus API

```
bw get item "Prometheus Agent RemoteWrite
```

## Tips & Tricks

no GPU support in WSL

```
export LIBGL_ALWAYS_SOFTWARE=1
```

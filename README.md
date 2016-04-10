[![Download on GoBuilder](http://badge.luzifer.io/v1/badge?title=Download%20on&text=GoBuilder)](https://gobuilder.me/github.com/Luzifer/ediplug_ctrl)
[![License: Apache v2.0](https://badge.luzifer.io/v1/badge?color=5d79b5&title=license&text=Apache+v2.0)](http://www.apache.org/licenses/LICENSE-2.0)
[![GoDoc reference](http://badge.luzifer.io/v1/badge?color=5d79b5&title=godoc&text=reference)](https://godoc.org/github.com/Luzifer/ediplug_ctrl/ediplug)
[![Go Report Card](https://goreportcard.com/badge/github.com/Luzifer/ediplug_ctrl)](https://goreportcard.com/report/github.com/Luzifer/ediplug_ctrl)

# Luzifer / ediplug\_ctrl

`ediplug_ctrl` is a small webserver to wrap some amount of [EdiPlug SP2101W](http://www.edimax.com/edimax/merchandise/merchandise_detail/data/edimax/au/home_automation_smart_plug/sp-1101w/) smart plugs. It is capable of fetching metrics from those plugs and to set the state through a simple API instead of messing with XML on the controlling side.

## Usage

### Starting

```bash
# ediplug_ctrl --help
Usage of ./ediplug_ctrl:
      --ip=[]: IPs of plugs to monitor / control
      --listen=":3000": Address to listen on for HTTP interface
      --password="1234": Password of the plugs
      --poll=10: Poll every N seconds
      --version[=false]: Show version and exit
```

You can run the `ediplug_ctrl` using docker or as a [single binary](https://gobuilder.me/github.com/Luzifer/ediplug_ctrl):

```bash
# docker run luzifer/ediplug_ctrl --ip=10.0.0.111

# ./ediplug_ctrl --ip=10.0.0.111
```

### API

The API exposes following methods:

- `/metrics` - Metrics endpoint to be fetched by a Prometheus instance
- `/switch/<plug name>/<on/off>` - Control the plug via its name you set in the settings

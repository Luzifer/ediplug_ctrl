package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/Luzifer/ediplug_ctrl/ediplug"
	"github.com/Luzifer/rconfig"
	"github.com/cenkalti/backoff"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/robfig/cron"
)

var (
	cfg = struct {
		ShowVersion  bool     `flag:"version" default:"false" description:"Show version and exit"`
		PlugIPs      []string `flag:"ip" default:"" description:"IPs of plugs to monitor / control"`
		PollInterval int      `flag:"poll" default:"10" description:"Poll every N seconds"`
		PlugPassword string   `flag:"password" default:"1234" description:"Password of the plugs"`
		Listen       string   `flag:"listen" default:":3000" description:"Address to listen on for HTTP interface"`
	}{}

	version = "dev"

	metrics = map[string]plugMetrics{}
	plugs   = map[string]string{}

	defaultBackoff = backoff.NewExponentialBackOff()
)

type plugMetrics struct {
	Activated     prometheus.Gauge
	NowCurrent    prometheus.Gauge
	NowPower      prometheus.Gauge
	DailyEnergy   prometheus.Gauge
	WeeklyEnergy  prometheus.Gauge
	MonthlyEnergy prometheus.Gauge
}

func init() {
	rconfig.Parse(&cfg)

	if cfg.ShowVersion {
		fmt.Printf("EdiPlug Control %s", version)
		os.Exit(0)
	}

	if len(cfg.PlugIPs) == 0 || reflect.DeepEqual(cfg.PlugIPs, []string{""}) {
		rconfig.Usage()
		os.Exit(0)
	}

	defaultBackoff.MaxElapsedTime = 5 * time.Second
}

func main() {
	for _, plugIP := range cfg.PlugIPs {
		c := &ediplug.GetSystemInfoCommand{}
		if err := backoff.Retry(func() error {
			return ediplug.ExecuteCommand(c, plugIP, cfg.PlugPassword)
		}, defaultBackoff); err != nil {
			log.Printf("Unable to fetch system information for plug '%s', not fetching data.", plugIP)
			continue
		}

		commonLabels := prometheus.Labels{
			"system_name":      c.SystemName,
			"mac_address":      c.MacAddress,
			"firmware_version": c.FirmwareVersion,
			"model":            c.Model,
		}

		plugs[c.SystemName] = plugIP
		metrics[plugIP] = plugMetrics{
			Activated: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   "ediplug",
				Name:        "activated",
				Help:        "0 if switched off, 1 if switched on",
				ConstLabels: commonLabels,
			}),
			NowCurrent: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   "ediplug",
				Name:        "now_current",
				Help:        "Current in Ampere fetched last iteration",
				ConstLabels: commonLabels,
			}),
			NowPower: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   "ediplug",
				Name:        "now_power",
				Help:        "Power in Watt fetched last iteration",
				ConstLabels: commonLabels,
			}),
			DailyEnergy: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   "ediplug",
				Name:        "daily_energy",
				Help:        "Energy used within last day, measured in kWh",
				ConstLabels: commonLabels,
			}),
			WeeklyEnergy: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   "ediplug",
				Name:        "weekly_energy",
				Help:        "Energy used within last week, measured in kWh",
				ConstLabels: commonLabels,
			}),
			MonthlyEnergy: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace:   "ediplug",
				Name:        "monthly_energy",
				Help:        "Energy used within last month, measured in kWh",
				ConstLabels: commonLabels,
			}),
		}

		prometheus.MustRegister(metrics[plugIP].Activated)
		prometheus.MustRegister(metrics[plugIP].DailyEnergy)
		prometheus.MustRegister(metrics[plugIP].MonthlyEnergy)
		prometheus.MustRegister(metrics[plugIP].NowCurrent)
		prometheus.MustRegister(metrics[plugIP].NowPower)
		prometheus.MustRegister(metrics[plugIP].WeeklyEnergy)
	}

	fetchMetrics()

	c := cron.New()
	c.AddFunc(fmt.Sprintf("@every %ds", cfg.PollInterval), fetchMetrics)
	c.Start()

	r := mux.NewRouter()
	r.Handle("/metrics", prometheus.Handler())
	r.HandleFunc("/switch/{system}/{state}", handlePlugSwitch)
	http.ListenAndServe(cfg.Listen, r)
}

func fetchMetrics() {
	for i := range cfg.PlugIPs {
		go func(plugIP string) {
			ce := &ediplug.GetEnergyCommand{}

			if err := backoff.Retry(func() error {
				return ediplug.ExecuteCommand(ce, plugIP, cfg.PlugPassword)
			}, defaultBackoff); err != nil {
				log.Printf("Unable to fetch metrics for plug '%s'", plugIP)
				return
			}

			metrics[plugIP].DailyEnergy.Set(ce.DailyEnergy)
			metrics[plugIP].MonthlyEnergy.Set(ce.MonthlyEnergy)
			metrics[plugIP].NowCurrent.Set(ce.NowCurrent)
			metrics[plugIP].NowPower.Set(ce.NowPower)
			metrics[plugIP].WeeklyEnergy.Set(ce.WeeklyEnergy)

			ca := &ediplug.GetStateCommand{}
			if err := backoff.Retry(func() error {
				return ediplug.ExecuteCommand(ca, plugIP, cfg.PlugPassword)
			}, defaultBackoff); err != nil {
				log.Printf("Unable to fetch acivation status for plug '%s'", plugIP)
				return
			}

			switch ca.CurrentState {
			case "ON":
				metrics[plugIP].Activated.Set(1)
			case "OFF":
				metrics[plugIP].Activated.Set(0)
			default:
				log.Printf("Got unexpected activation status for plug '%s': %s", plugIP, ca.CurrentState)
			}
		}(cfg.PlugIPs[i])
	}
}

func handlePlugSwitch(res http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	ip, ok := plugs[vars["system"]]
	if !ok {
		http.Error(res, "Plug not found.", http.StatusNotFound)
		return
	}

	stateRequest := &ediplug.SetStateCommand{}

	switch vars["state"] {
	case "on":
		stateRequest.DesiredState = "ON"
	case "off":
		stateRequest.DesiredState = "OFF"
	default:
		http.Error(res, "Status not possible.", http.StatusNotAcceptable)
		return
	}

	if err := backoff.Retry(func() error {
		return ediplug.ExecuteCommand(stateRequest, ip, cfg.PlugPassword)
	}, defaultBackoff); err != nil {
		http.Error(res, fmt.Sprintf("An error occurred while setting state: %s", err), http.StatusInternalServerError)
		return
	}

	http.Error(res, "OK", http.StatusOK)
}

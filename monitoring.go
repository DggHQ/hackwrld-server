package main

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Monitor struct {
	UpgradeRequests *prometheus.CounterVec
}

func (m *Monitor) Run() {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":80", nil))
}

func (m *Monitor) Init() *Monitor {
	m.UpgradeRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "hackwrld_upgrade_request",
		Help: "Total Upgrade Requests"}, []string{"id", "nick", "component"})

	prometheus.MustRegister(m.UpgradeRequests)

	return m
}

// Copyright 2021 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strings"

	"github.com/prometheus-community/ipmi_exporter/freeipmi"
)

const (
	SELCollectorName CollectorName = "sel"
)

var (
	selCpuLeakStatusDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "sel", "cpu_leak_status"),
		"Current Assertion Event for CPU Leak Status.",
		[]string{},
		nil,
	)
)

type SELCollector struct{}

func (c SELCollector) Name() CollectorName {
	return SELCollectorName
}

func (c SELCollector) Cmd() string {
	return "ipmi-sel"
}

func (c SELCollector) Args() []string {
	return []string{"-vv"}
}

func (c SELCollector) Collect(result freeipmi.Result, ch chan<- prometheus.Metric, target ipmiTarget) (int, error) {
	if result.Err != nil {
		err := level.Error(logger).Log(
			"msg", "Failed to collect CPU leak data", "target", targetName(target.host), "error", result.Err)
		if err != nil {
			return 0, err
		}
		return 0, err
	}

	stringItem := strings.Split(string(result.Output), "\n")
	var cpuLeakCount = 0
	for _, value := range stringItem {
		level.Error(logger).Log(value)
		if strings.Contains(value, "CPU_Leak_Status") && strings.Contains(value, "Assertion") {
			cpuLeakCount++
		}
	}
	level.Error(logger).Log("cpuLeakCount", cpuLeakCount)
	ch <- prometheus.MustNewConstMetric(
		selCpuLeakStatusDesc,
		prometheus.GaugeValue,
		float64(cpuLeakCount),
	)
	level.Error(logger).Log("return")
	return 1, nil
}

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
	selGpuLeakStatusDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "sel", "gpu_leak_status"),
		"Current Assertion Event for GPU Leak Status.",
		[]string{},
		nil,
	)
	selMBLeakStatusDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "sel", "mb_leak_status"),
		"Current Assertion Event for MB Leak Status.",
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
			"msg", "Failed to collect GPU leak data", "target", targetName(target.host), "error", result.Err)
		if err != nil {
			return 0, err
		}
		return 0, err
	}

	stringItem := strings.Split(string(result.Output), "\n")
	var gpuLeakCount = 0
	var mbLeakCount = 0
	for _, value := range stringItem {
		if strings.Contains(value, "GPU_Leak_Status") && strings.Contains(value, "Assertion") {
			gpuLeakCount++
		}
		if strings.Contains(value, "MB_Leak_Status") && strings.Contains(value, "Assertion") {
			mbLeakCount++
		}
	}
	ch <- prometheus.MustNewConstMetric(
		selGpuLeakStatusDesc,
		prometheus.GaugeValue,
		float64(gpuLeakCount),
	)
	ch <- prometheus.MustNewConstMetric(
		selMBLeakStatusDesc,
		prometheus.GaugeValue,
		float64(mbLeakCount),
	)
	return 1, nil
}

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

package freeipmi

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

var (
	ipmiDCMICurrentPowerRegex         = regexp.MustCompile(`^Current Power\s*:\s*(?P<value>[0-9.]*)\s*Watts.*`)
	ipmiChassisPowerRegex             = regexp.MustCompile(`^System Power\s*:\s(?P<value>.*)`)
	bmcInfoFirmwareRevisionRegex      = regexp.MustCompile(`^Firmware Revision\s*:\s*(?P<value>[0-9.]*).*`)
	bmcInfoSystemFirmwareVersionRegex = regexp.MustCompile(`^System Firmware Version\s*:\s*(?P<value>[0-9.]*).*`)
	bmcInfoManufacturerIDRegex        = regexp.MustCompile(`^Manufacturer ID\s*:\s*(?P<value>.*)`)
)

// Result represents the outcome of a call to one of the FreeIPMI tools.
// It can be used with other functions in this package to extract data.
type Result struct {
	Output []byte
	Err    error
}

// SensorData represents the reading of a single sensor.
type SensorData struct {
	ID    int64
	Name  string
	Type  string
	State string
	Value float64
	Unit  string
	Event string
}

// EscapePassword escapes a password so that the result is suitable for usage in a
// FreeIPMI config file.
func EscapePassword(password string) string {
	return strings.Replace(password, "#", "\\#", -1)
}

func pipeName() (string, error) {
	randBytes := make([]byte, 16)
	_, err := rand.Read(randBytes)
	if err != nil {
		return "", err
	}
	return filepath.Join(os.TempDir(), "ipmi_exporter-"+hex.EncodeToString(randBytes)), nil
}

func contains(s []int64, elm int64) bool {
	for _, a := range s {
		if a == elm {
			return true
		}
	}
	return false
}

func getValue(ipmiOutput []byte, regex *regexp.Regexp) (string, error) {
	for _, line := range strings.Split(string(ipmiOutput), "\n") {
		match := regex.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		for i, name := range regex.SubexpNames() {
			if name != "value" {
				continue
			}
			return match[i], nil
		}
	}
	return "", fmt.Errorf("could not find value in output: %s", string(ipmiOutput))
}

func freeIpmiConfigPipe(config string, logger log.Logger) (string, error) {
	content := []byte(config)
	pipe, err := pipeName()
	if err != nil {
		return "", err
	}
	err = syscall.Mkfifo(pipe, 0600)
	if err != nil {
		return "", err
	}

	go func(file string, data []byte) {
		f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModeNamedPipe)
		if err != nil {
			err := level.Error(logger).Log("msg", "Error opening pipe", "error", err)
			if err != nil {
				return
			}
		}
		if _, err := f.Write(data); err != nil {
			err := level.Error(logger).Log("msg", "Error writing config to pipe", "error", err)
			if err != nil {
				return
			}
		}
		f.Close()
	}(pipe, content)
	return pipe, nil
}

func Execute(cmd string, args []string, config string, target string, logger log.Logger) Result {
	pipe, err := freeIpmiConfigPipe(config, logger)
	if err != nil {
		return Result{nil, err}
	}
	defer func() {
		if err := os.Remove(pipe); err != nil {
			err := level.Error(logger).Log("msg", "Error deleting named pipe", "error", err)
			if err != nil {
				return
			}
		}
	}()

	args = append(args, "--config-file", pipe)
	if target != "" {
		args = append(args, "-h", target)
	}

	err = level.Debug(logger).Log("msg", "Executing", "command", cmd, "args", fmt.Sprintf("%+v", args))
	if err != nil {
		return Result{}
	}
	out, err := exec.Command(cmd, args...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("error running %s: %s", cmd, err)
	}
	return Result{out, err}
}

func GetSensorData(ipmiOutput Result, excludeSensorIds []int64) ([]SensorData, error) {
	var result []SensorData

	if ipmiOutput.Err != nil {
		return result, fmt.Errorf("%s: %s", ipmiOutput.Err, ipmiOutput.Output)
	}

	r := csv.NewReader(bytes.NewReader(ipmiOutput.Output))
	fields, err := r.ReadAll()
	if err != nil {
		return result, err
	}

	for _, line := range fields {
		var data SensorData

		data.ID, err = strconv.ParseInt(line[0], 10, 64)
		if err != nil {
			return result, err
		}
		if contains(excludeSensorIds, data.ID) {
			continue
		}

		data.Name = line[1]
		data.Type = line[2]
		data.State = line[3]

		value := line[4]
		if value != "N/A" {
			data.Value, err = strconv.ParseFloat(value, 64)
			if err != nil {
				return result, err
			}
		} else {
			data.Value = math.NaN()
		}

		data.Unit = line[5]
		data.Event = strings.Trim(line[6], "'")

		result = append(result, data)
	}
	return result, err
}

func GetCurrentPowerConsumption(ipmiOutput Result) (float64, error) {
	if ipmiOutput.Err != nil {
		return -1, fmt.Errorf("%s: %s", ipmiOutput.Err, ipmiOutput.Output)
	}
	value, err := getValue(ipmiOutput.Output, ipmiDCMICurrentPowerRegex)
	if err != nil {
		return -1, err
	}
	return strconv.ParseFloat(value, 64)
}

func GetChassisPowerState(ipmiOutput Result) (float64, error) {
	if ipmiOutput.Err != nil {
		return -1, fmt.Errorf("%s: %s", ipmiOutput.Err, ipmiOutput.Output)
	}
	value, err := getValue(ipmiOutput.Output, ipmiChassisPowerRegex)
	if err != nil {
		return -1, err
	}
	if value == "on" {
		return 1, err
	}
	return 0, err
}

func GetBMCInfoFirmwareRevision(ipmiOutput Result) (string, error) {
	// Workaround for an issue described here: https://github.com/prometheus-community/ipmi_exporter/issues/57
	// The command may fail, but produce usable output (minus the system firmware revision).
	// Try to recover gracefully from that situation by first trying to parse the output, and only
	// raise the initial error if that also fails.
	value, err := getValue(ipmiOutput.Output, bmcInfoFirmwareRevisionRegex)
	if err != nil {
		if ipmiOutput.Err != nil {
			return "", fmt.Errorf("%s: %s", ipmiOutput.Err, ipmiOutput.Output)
		}
	}
	return value, err
}

func GetBMCInfoManufacturerID(ipmiOutput Result) (string, error) {
	// Workaround for an issue described here: https://github.com/prometheus-community/ipmi_exporter/issues/57
	// The command may fail, but produce usable output (minus the system firmware revision).
	// Try to recover gracefully from that situation by first trying to parse the output, and only
	// raise the initial error if that also fails.
	value, err := getValue(ipmiOutput.Output, bmcInfoManufacturerIDRegex)
	if err != nil {
		if ipmiOutput.Err != nil {
			return "", fmt.Errorf("%s: %s", ipmiOutput.Err, ipmiOutput.Output)
		}
	}
	return value, err
}

func GetBMCInfoSystemFirmwareVersion(ipmiOutput Result) (string, error) {
	if ipmiOutput.Err != nil {
		return "", fmt.Errorf("%s: %s", ipmiOutput.Err, ipmiOutput.Output)
	}
	return getValue(ipmiOutput.Output, bmcInfoSystemFirmwareVersionRegex)
}

func GetRawOctets(ipmiOutput Result) ([]string, error) {
	if ipmiOutput.Err != nil {
		return nil, fmt.Errorf("%s: %s", ipmiOutput.Err, ipmiOutput.Output)
	}
	strOutput := strings.Trim(string(ipmiOutput.Output), " \r\n")
	if !strings.HasPrefix(strOutput, "rcvd: ") {
		return nil, fmt.Errorf("unexpected raw response: %s", strOutput)
	}
	octects := strings.Split(strOutput[6:], " ")
	return octects, nil
}

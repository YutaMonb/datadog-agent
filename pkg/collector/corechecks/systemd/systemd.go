// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2019 Datadog, Inc.

package systemd

import (
	"fmt"
	"regexp"

	"github.com/DataDog/datadog-agent/pkg/aggregator"
	"github.com/DataDog/datadog-agent/pkg/autodiscovery/integration"
	"github.com/DataDog/datadog-agent/pkg/collector/check"
	"github.com/DataDog/datadog-agent/pkg/util/log"
	"github.com/coreos/go-systemd/dbus"
	"gopkg.in/yaml.v2"

	core "github.com/DataDog/datadog-agent/pkg/collector/corechecks"
)

const systemdCheckName = "systemd"

// For testing purpose
var (
	dbusNew       = dbus.New
	connListUnits = func(c *dbus.Conn) ([]dbus.UnitStatus, error) { return c.ListUnits() }
	connClose     = func(c *dbus.Conn) { c.Close() }
)

// SystemdCheck doesn't need additional fields
type SystemdCheck struct {
	core.CheckBase
	config systemdConfig
}

type systemdInstanceConfig struct {
	UnitNames         []string `yaml:"unit_names"`
	UnitRegexStrings  []string `yaml:"unit_regex"`
	UnitRegexPatterns []*regexp.Regexp
}

type systemdInitConfig struct{}

type systemdConfig struct {
	instance systemdInstanceConfig
	initConf systemdInitConfig
}

// Run executes the check
func (c *SystemdCheck) Run() error {

	sender, err := aggregator.GetSender(c.ID())
	if err != nil {
		return err
	}

	conn, err := dbusNew()
	if err != nil {
		log.Error("New Connection Err: ", err)
		return err
	}
	defer connClose(conn)

	// Overall Unit Metrics
	units, err := connListUnits(conn)
	if err != nil {
		fmt.Println("ListUnits Err: ", err)
		return err
	}

	activeUnitCounter := 0
	for _, unit := range units {
		log.Debugf("[unit] %s: ActiveState=%s, SubState=%s", unit.Name, unit.ActiveState, unit.SubState)
		if unit.ActiveState == "active" {
			activeUnitCounter++
		}
	}

	sender.Gauge("systemd.unit.active.count", float64(activeUnitCounter), "", nil)

	sender.Gauge("systemd.unit.cpu", 1, "", nil)
	sender.Commit()

	return nil
}

// Configure configures the network checks
func (c *SystemdCheck) Configure(rawInstance integration.Data, rawInitConfig integration.Data) error {
	err := c.CommonConfigure(rawInstance)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(rawInitConfig, &c.config.initConf)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(rawInstance, &c.config.instance)
	if err != nil {
		return err
	}

	log.Warnf("[DEV] c.config.instance.UnitNames: %v", c.config.instance.UnitNames)
	log.Warnf("[DEV] c.config.instance.UnitRegexStrings: %v", c.config.instance.UnitRegexStrings)

	for _, regexString := range c.config.instance.UnitRegexStrings {
		pattern, err := regexp.Compile(regexString)
		if err != nil {
			log.Errorf("Failed to parse systemd check option unit_regex: %s", err)
		} else {
			c.config.instance.UnitRegexPatterns = append(c.config.instance.UnitRegexPatterns, pattern)
		}
	}
	log.Warnf("[DEV] c.config.instance.UnitRegexPatterns: %v", c.config.instance.UnitRegexPatterns)
	return nil
}

func systemdFactory() check.Check {
	return &SystemdCheck{
		CheckBase: core.NewCheckBase(systemdCheckName),
	}
}

func init() {
	core.RegisterCheck(systemdCheckName, systemdFactory)
}

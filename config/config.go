package config

import (
	"fmt"
	"regexp"
	"github.com/pepabo/go-netapp/netapp"
	"github.com/prometheus/common/log"
	"github.com/creasty/defaults"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"sync"
	"time"
)

type Config struct {
	Devices map[string]DeviceConfig `yaml:"devices"`
}

type SafeConfig struct {
	sync.RWMutex
	C *Config
}

type DeviceConfig struct {
	Group      string             `yaml:"group"`
	Username   string             `yaml:"username"`
	Password   string             `yaml:"password"`
	Debug      bool               `yaml:"debug"`
	PerfData []string             `yaml:"perfdata" default:"[\"system\", \"system:node\", \"nfsv3\", \"nfsv3:node\", \"lif\", \"lun\", \"aggregate\", \"disk\", \"workload\", \"processor\", \"processor:node\", \"volume:node\", \"volume:vserver\", \"volume\"]"`
	Filter     MetricFilterConfig `yaml:"filter"`
	FilterCompiled *MetricFilterCompiled
}

type MetricFilterConfig struct {
	Include  []string            `yaml:"include" default:"[]"`
	Exclude  []string            `yaml:"exclude" default:"[]"`
}

type MetricFilterCompiled struct {
	Include  []*regexp.Regexp
	Exclude  []*regexp.Regexp
}

func CompileFilter(metric *MetricFilterConfig) *MetricFilterCompiled {
	include := make([]*regexp.Regexp, len(metric.Include))
	exclude := make([]*regexp.Regexp, len(metric.Exclude))

	for i, str := range metric.Include {
		include[i], _ = regexp.Compile(str)
	}
	for i, str := range metric.Exclude {
		exclude[i], _ = regexp.Compile(str)
	}

	return &MetricFilterCompiled{
		Include: include,
		Exclude: exclude,
	}
}


// validate a metric against the include and exclude field
// return true if correspondig should be exposed
func (mf *MetricFilterCompiled) MetricValidate(metricName string) bool {
	isIncluded := false
	if len(mf.Include) == 0 {
		isIncluded = true
	} else {
		for _, expr := range mf.Include {
			if expr.MatchString(metricName) {
				isIncluded = true
				break
			}
		}
	}
	if !isIncluded {
		return false
	}
	// now, we know that the include pattern have matched, do again with exclude pattern
	if len(mf.Exclude) == 0 {
		return true
	}
	for _, expr := range mf.Exclude {
		if expr.MatchString(metricName)  {
			return false
		}
	}
	return true
}

func (sc *SafeConfig) ReloadConfig(configFile string) error {
	var c = &Config{}

	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Errorf("Error reading config file: %s", err)
		return err
	}
	if err := yaml.Unmarshal(yamlFile, c); err != nil {
		log.Errorf("Error parsing config file: %s", err)
		return err
	}

	sc.Lock()
	sc.C = c
	sc.Unlock()

	log.Infoln("Loaded config file")
	return nil
}

func (sc *SafeConfig) DeviceConfigForTarget(target string) (*DeviceConfig, error) {
	sc.Lock()
	defer sc.Unlock()
	if deviceConfig, ok := sc.C.Devices[target]; ok {
		defaults.Set(&deviceConfig)
		return &DeviceConfig{
			Group:    deviceConfig.Group,
			Username: deviceConfig.Username,
			Password: deviceConfig.Password,
			Debug:    deviceConfig.Debug,
                        PerfData: deviceConfig.PerfData,
			Filter:   deviceConfig.Filter,
			FilterCompiled: CompileFilter(&deviceConfig.Filter),
		}, nil
	}
	if deviceConfig, ok := sc.C.Devices["default"]; ok {
		return &DeviceConfig{
			Group:    deviceConfig.Group,
			Username: deviceConfig.Username,
			Password: deviceConfig.Password,
			Debug:    deviceConfig.Debug,
                        PerfData: deviceConfig.PerfData,
			Filter:   deviceConfig.Filter,
			FilterCompiled: CompileFilter(&deviceConfig.Filter),
		}, nil
	}
	return &DeviceConfig{}, fmt.Errorf("no credentials found for target %s", target)
}

func NewNetappClient(host string, deviceConfig *DeviceConfig) (string, *netapp.Client) {

	_url := "https://%s/servlets/netapp.servlets.admin.XMLrequest_filer"
	url := fmt.Sprintf(_url, host)

	version := "1.130"

	opts := &netapp.ClientOptions{
		BasicAuthUser:     deviceConfig.Username,
		BasicAuthPassword: deviceConfig.Password,
		SSLVerify:         false,
		Debug:             deviceConfig.Debug,
		Timeout:           30 * time.Second,
	}
	netappClient := netapp.NewClient(url, version, opts)
	return deviceConfig.Group, netappClient
}

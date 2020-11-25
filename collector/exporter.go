package collector

import (
	"sync"
	"time"

	"github.com/jenningsloy318/netapp_exporter/collector/metrics"
	"github.com/jenningsloy318/netapp_exporter/collector/metrics/perf"
	"github.com/jenningsloy318/netapp_exporter/collector/metrics/variables"
	"github.com/jenningsloy318/netapp_exporter/config"
	"github.com/pepabo/go-netapp/netapp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

// Metric name parts.
const (
	// Subsystem(s).
	exporter = "exporter"
)

// Metric descriptors.
var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(variables.Namespace, exporter, "collector_duration_seconds"),
		"Collector time duration.",
		[]string{"collector"}, nil,
	)
)

// Exporter collects NetAPP metrics. It implements prometheus.Collector.
type Exporter struct {
	netappClient *netapp.Client
	error        prometheus.Gauge
	scrapers     []Scraper
	totalScrapes prometheus.Counter
	scrapeErrors *prometheus.CounterVec
	netappUp     prometheus.Gauge
	deviceConfig *config.DeviceConfig
}

var scrapers = []Scraper{
	metrics.ScrapeSystem{},
	metrics.ScrapeAggr{},
	metrics.ScrapeVserver{},
	metrics.ScrapeVolume{},
	metrics.ScrapeLun{},
	metrics.ScrapeSnapshot{},
	metrics.ScrapeStorageDisk{},
}

func New(Groupname string, netappClient *netapp.Client, deviceConfig *config.DeviceConfig) *Exporter {
	variables.BaseLabelValues[0] = Groupname
	return &Exporter{
		netappClient: netappClient,
		scrapers:     append(scrapers, perf.New(deviceConfig.PerfData)),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: variables.Namespace,
			Subsystem: exporter,
			Name:      "scrapes_total",
			Help:      "Total number of times NetAPP was scraped for metrics.",
		}),
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: variables.Namespace,
			Subsystem: exporter,
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occurred scraping a NetAPP.",
		}, []string{"collector"}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: variables.Namespace,
			Subsystem: exporter,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from NetAPP resulted in an error (1 for error, 0 for success).",
		}),

		netappUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: variables.Namespace,
			Name:      "up",
			Help:      "Whether the NetAPP server is up.",
		}),

		deviceConfig: deviceConfig,
	}
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	// We cannot know in advance what metrics the exporter will generate
	// from NetAPP. So we use the poor man's describe method: Run a collect
	// and send the descriptors of all the collected metrics. The problem
	// here is that we need to connect to the NetAPP . If it is currently
	// unavailable, the descriptors will be incomplete. Since this is a
	// stand-alone exporter and not used as a library within other code
	// implementing additional metrics, the worst that can happen is that we
	// don't detect inconsistent metrics created by this exporter
	// itself. Also, a change in the monitored NetAPP instance may change the
	// exported metrics during the runtime of the exporter.

	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	e.Collect(metricCh)
	close(metricCh)
	<-doneCh
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {

	e.scrape(ch)
	ch <- e.totalScrapes
	ch <- e.error
	ch <- e.netappUp
	e.scrapeErrors.Collect(ch)
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()
	scrapeTime := time.Now()
	e.netappUp.Set(0)
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "connection")

	if clusterIdentity, ok := GetClusterIdentity(e.netappClient); ok {

		e.netappUp.Set(1)
		variables.BaseLabelValues[1] = clusterIdentity["clusterName"]

	} else {
		e.netappUp.Set(0)
		return
	}

	wg := &sync.WaitGroup{}
	defer wg.Wait()
	for _, scraper := range e.scrapers {
		wg.Add(1)
		go func(scraper Scraper) {
			defer wg.Done()
			log.Debug("start scraping" + scraper.Name())
			label := "collect." + scraper.Name()
			scrapeTime := time.Now()
			if err := scraper.Scrape(e.netappClient, ch); err != nil {
				log.Errorln("Error scraping for "+label+":", err)
				e.scrapeErrors.WithLabelValues(label).Inc()
				e.error.Set(1)
			}

			ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), label)
		}(scraper)
	}
}

func GetClusterIdentity(netappClient *netapp.Client) (map[string]string, bool) {

	clusterIdentity := make(map[string]string)
	ops := &netapp.ClusterIdentityOptions{
		DesiredAttributes: &netapp.ClusterIdentityInfo{},
	}

	l, _, err := netappClient.ClusterIdentity.List(ops)
	if err !=nil  {
		log.Infof("error when getting ClusterIdentity, %s",err)
		return clusterIdentity, false
	}
	clusterIdentity["clusterName"] = l.Results.ClusterIdentityInfo[0].ClusterName
	clusterIdentity["clusterSerialNumber"] = l.Results.ClusterIdentityInfo[0].ClusterSerialNumber
	clusterIdentity["clusterLocation"] = l.Results.ClusterIdentityInfo[0].ClusterLocation
	return clusterIdentity,true 
}

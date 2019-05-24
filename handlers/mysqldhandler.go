package handlers

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/prometheus/mysqld_exporter/collector"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/ini.v1"
	"io/ioutil"
	"net/http"
	"path"
)

var Dsn = kingpin.Flag(
	"mysqld.dsn",
	"Data source name",
).Default(path.Join("root:123qwe@(localhost:3309)/")).String()

// scrapers lists all possible collection methods and if they should be enabled by default.
var Scrapers = map[collector.Scraper]bool{
	collector.ScrapeGlobalStatus{}:                    true,
	collector.ScrapeGlobalVariables{}:                 true,
	collector.ScrapeSlaveStatus{}:                     true,
	collector.ScrapeProcesslist{}:                     false,
	collector.ScrapeTableSchema{}:                     true,
	collector.ScrapeInfoSchemaInnodbTablespaces{}:     false,
	collector.ScrapeInnodbMetrics{}:                   false,
	collector.ScrapeAutoIncrementColumns{}:            false,
	collector.ScrapeBinlogSize{}:                      false,
	collector.ScrapePerfTableIOWaits{}:                false,
	collector.ScrapePerfIndexIOWaits{}:                false,
	collector.ScrapePerfTableLockWaits{}:              false,
	collector.ScrapePerfEventsStatements{}:            false,
	collector.ScrapePerfEventsWaits{}:                 false,
	collector.ScrapePerfFileEvents{}:                  false,
	collector.ScrapePerfFileInstances{}:               false,
	collector.ScrapePerfReplicationGroupMemberStats{}: false,
	collector.ScrapeUserStat{}:                        false,
	collector.ScrapeClientStat{}:                      false,
	collector.ScrapeTableStat{}:                       false,
	collector.ScrapeInnodbCmp{}:                       false,
	collector.ScrapeInnodbCmpMem{}:                    false,
	collector.ScrapeQueryResponseTime{}:               false,
	collector.ScrapeEngineTokudbStatus{}:              false,
	collector.ScrapeEngineInnodbStatus{}:              false,
	collector.ScrapeHeartbeat{}:                       false,
	collector.ScrapeSlaveHosts{}:                      false,
}

func ParseMycnf(config interface{}) (string, error) {
	var dsn string
	opts := ini.LoadOptions{
		// MySQL ini file can have boolean keys.
		AllowBooleanKeys: true,
	}
	cfg, err := ini.LoadSources(opts, config)
	if err != nil {
		return dsn, fmt.Errorf("failed reading ini file: %s", err)
	}
	user := cfg.Section("client").Key("user").String()
	password := cfg.Section("client").Key("password").String()
	if (user == "") || (password == "") {
		return dsn, fmt.Errorf("no user or password specified under [client] in %s", config)
	}
	host := cfg.Section("client").Key("host").MustString("localhost")
	port := cfg.Section("client").Key("port").MustUint(3306)
	socket := cfg.Section("client").Key("socket").String()
	if socket != "" {
		dsn = fmt.Sprintf("%s:%s@unix(%s)/", user, password, socket)
	} else {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/", user, password, host, port)
	}
	sslCA := cfg.Section("client").Key("ssl-ca").String()
	sslCert := cfg.Section("client").Key("ssl-cert").String()
	sslKey := cfg.Section("client").Key("ssl-key").String()
	if sslCA != "" {
		if tlsErr := customizeTLS(sslCA, sslCert, sslKey); tlsErr != nil {
			tlsErr = fmt.Errorf("failed to register a custom TLS configuration for mysql dsn: %s", tlsErr)
			return dsn, tlsErr
		}
		dsn = fmt.Sprintf("%s?tls=custom", dsn)
	}

	log.Debugln(dsn)
	return dsn, nil
}

func customizeTLS(sslCA string, sslCert string, sslKey string) error {
	var tlsCfg tls.Config
	caBundle := x509.NewCertPool()
	pemCA, err := ioutil.ReadFile(sslCA)
	if err != nil {
		return err
	}
	if ok := caBundle.AppendCertsFromPEM(pemCA); ok {
		tlsCfg.RootCAs = caBundle
	} else {
		return fmt.Errorf("failed parse pem-encoded CA certificates from %s", sslCA)
	}
	if sslCert != "" && sslKey != "" {
		certPairs := make([]tls.Certificate, 0, 1)
		keypair, err := tls.LoadX509KeyPair(sslCert, sslKey)
		if err != nil {
			return fmt.Errorf("failed to parse pem-encoded SSL cert %s or SSL key %s: %s",
				sslCert, sslKey, err)
		}
		certPairs = append(certPairs, keypair)
		tlsCfg.Certificates = certPairs
	}
	mysql.RegisterTLSConfig("custom", &tlsCfg)
	return nil
}

func init() {
	prometheus.MustRegister(version.NewCollector("mysqld_exporter"))
}

func NewMysqldHandler(metrics collector.Metrics, scrapers []collector.Scraper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filteredScrapers := scrapers
		params := r.URL.Query()["collect[]"]
		log.Debugln("collect query:", params)

		// Check if we have some "collect[]" query parameters.
		if len(params) > 0 {
			filters := make(map[string]bool)
			for _, param := range params {
				filters[param] = true
			}

			filteredScrapers = nil
			for _, scraper := range scrapers {
				if filters[scraper.Name()] {
					filteredScrapers = append(filteredScrapers, scraper)
				}
			}
		}

		registry := prometheus.NewRegistry()
		registry.MustRegister(collector.New(*Dsn, metrics, filteredScrapers))

		gatherers := prometheus.Gatherers{
			//prometheus.DefaultGatherer,
			registry,
		}
		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}
}

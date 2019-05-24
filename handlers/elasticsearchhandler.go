package handlers

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	glog "github.com/go-kit/kit/log"
	"github.com/justwatchcom/elasticsearch_exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	Name = "elasticsearch_exporter"
	esURI = kingpin.Flag(
		"es.uri",
		"HTTP API address of an Elasticsearch node.",
	).Default("http://localhost:9200").String()

	esTimeout = kingpin.Flag(
		"es.timeout",
		"Timeout for trying to get stats from Elasticsearch.",
	).Default("5s").Duration()

	esAllNodes = kingpin.Flag(
		"es.all",
		"Export stats for all nodes in the cluster.",
	).Default("false").Bool()

	esExportIndices = kingpin.Flag(
		"es.indices",
		"Export stats for indices in the cluster.",
	).Default("false").Bool()

	esCA = kingpin.Flag(
		"es.ca",
		"Path to PEM file that conains trusted CAs for the Elasticsearch connection.",
	).Default("").String()

	esClientPrivateKey = kingpin.Flag(
		"es.client-private-key",
		"Path to PEM file that conains the private key for client auth when connecting to Elasticsearch.",
	).Default("").String()

	esClientCert = kingpin.Flag(
		"es.client-cert",
		"Path to PEM file that conains the corresponding cert for the private key to connect to Elasticsearch.",
	).Default("").String()
)

func NewElasticsearchHandler() http.Handler {
	logger := glog.NewLogfmtLogger(glog.NewSyncWriter(os.Stdout))
	logger = glog.With(logger,
		"ts", glog.DefaultTimestampUTC,
		"caller", glog.DefaultCaller,
	)

	esURL, err := url.Parse(*esURI)
	if err != nil {
		log.Warnln("failed to parse es.uri:", err)
	}

	// returns nil if not provided and falls back to simple TCP.
	tlsConfig := createTLSConfig(*esCA, *esClientCert, *esClientPrivateKey)

	httpClient := &http.Client{
		Timeout: *esTimeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	// version metric
	registry := prometheus.NewRegistry()
	versionMetric := version.NewCollector(Name)
	registry.MustRegister(versionMetric)
	registry.MustRegister(collector.NewClusterHealth(logger, httpClient, esURL))
	registry.MustRegister(collector.NewNodes(logger, httpClient, esURL, *esAllNodes))
	if *esExportIndices {
		registry.MustRegister(collector.NewIndices(logger, httpClient, esURL))
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return handler
}


func createTLSConfig(pemFile, pemCertFile, pemPrivateKeyFile string) *tls.Config {
	if len(pemFile) <= 0 {
		return nil
	}
	rootCerts, err := loadCertificatesFrom(pemFile)
	if err != nil {
		log.Fatalf("Couldn't load root certificate from %s. Got %s.", pemFile, err)
	}
	if len(pemCertFile) > 0 && len(pemPrivateKeyFile) > 0 {
		clientPrivateKey, err := loadPrivateKeyFrom(pemCertFile, pemPrivateKeyFile)
		if err != nil {
			log.Fatalf("Couldn't setup client authentication. Got %s.", err)
		}
		return &tls.Config{
			RootCAs:      rootCerts,
			Certificates: []tls.Certificate{*clientPrivateKey},
		}
	}
	return &tls.Config{
		RootCAs: rootCerts,
	}
}

func loadCertificatesFrom(pemFile string) (*x509.CertPool, error) {
	caCert, err := ioutil.ReadFile(pemFile)
	if err != nil {
		return nil, err
	}
	certificates := x509.NewCertPool()
	certificates.AppendCertsFromPEM(caCert)
	return certificates, nil
}

func loadPrivateKeyFrom(pemCertFile, pemPrivateKeyFile string) (*tls.Certificate, error) {
	privateKey, err := tls.LoadX509KeyPair(pemCertFile, pemPrivateKeyFile)
	if err != nil {
		return nil, err
	}
	return &privateKey, nil
}

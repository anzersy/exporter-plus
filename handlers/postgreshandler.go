package handlers

import (
	"exporter-plus/collector/postgres"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"github.com/prometheus/common/log"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	disableDefaultMetrics  = kingpin.Flag("disable-default-metrics", "Do not include default metrics.").Default("false").Envar("PG_EXPORTER_DISABLE_DEFAULT_METRICS").Bool()
	disableSettingsMetrics = kingpin.Flag("disable-settings-metrics", "Do not include pg_settings metrics.").Default("false").Envar("PG_EXPORTER_DISABLE_SETTINGS_METRICS").Bool()
	queriesPath            = kingpin.Flag("extend.query-path", "Path to custom queries to run.").Default("").Envar("PG_EXPORTER_EXTEND_QUERY_PATH").String()
	onlyDumpMaps           = kingpin.Flag("dumpmaps", "Do not run, simply dump the maps.").Bool()
	constantLabelsList     = kingpin.Flag("constantLabels", "A list of label=value separated by comma(,).").Default("").Envar("PG_EXPORTER_CONSTANT_LABELS").String()
)


func NewPostgresHandler() http.Handler {
	dsn := postgres.GetDataSources()
	if len(dsn) == 0 {
		log.Fatal("couldn't find environment variables describing the datasource to use")
	}

	exporter := postgres.NewPostgresExporter(dsn,
		postgres.DisableDefaultMetrics(*disableDefaultMetrics),
		postgres.DisableSettingsMetrics(*disableSettingsMetrics),
		postgres.WithUserQueriesPath(*queriesPath),
		postgres.WithConstantLabels(*constantLabelsList),
	)

	registry := prometheus.NewRegistry()
	registry.MustRegister(exporter)
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return handler
}
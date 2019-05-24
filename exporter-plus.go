package main

import (
	"exporter-plus/handlers"
	"fmt"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	mysqldcollector "github.com/prometheus/mysqld_exporter/collector"
	"github.com/prometheus/node_exporter/collector"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
	"sort"
	_ "net/http/pprof"
)

var (
	mysqlSwitch = kingpin.Flag(
		"mysql.switch",
		"Enable the mysql exporter",
	).Default("disable").String()
	redisSwitch = kingpin.Flag(
		"redis.switch",
		"Enable the redis exporter",
	).Default("disable").String()
	mongodbSwitch = kingpin.Flag(
		"mongodb.switch",
		"Enable the mongodb exporter",
	).Default("disable").String()
	elasticsearchSwitch = kingpin.Flag(
		"elastic.switch",
		"Enable the elasticsearch exporter",
	).Default("disable").String()
	kafkaSwitch = kingpin.Flag(
		"kafka.switch",
		"Enable the kafka exporter",
	).Default("disable").String()
	postgresSwitch = kingpin.Flag(
		"postgres.switch",
		"Enable the postgres exporter",
	).Default("disable").String()
)

func main() {

	listenAddress := kingpin.Flag("listen-address", "Address on which to expose metrics and web interface.").Default(":9100").String()
	log.Infoln("exporter-plus", version.Info())
	log.Infoln("Build context", version.BuildContext())
	scrapers := getMysqlExporterScrapers()

	handlers.GetKafkaFlag()

	parseFlags()
	// 初始化node exporter
	initNodeExporter()

	if *mysqlSwitch == "enable" {
		//初始化mysqld exporter
		enabledScrapers := initMysqlExporter(scrapers)
		http.HandleFunc("/mysqld", handlers.NewMysqldHandler(mysqldcollector.NewMetrics(), enabledScrapers))
	}

	if *redisSwitch == "enable" {
		//初始化redis exporter
		http.HandleFunc("/redis", handlers.NewRedisHandler().ServeHTTP)
	}

	if *mongodbSwitch == "enable" {
		//初始化mongodb exporter
		http.HandleFunc("/mongodb", handlers.NewMongodbHandler().ServeHTTP)
	}

	if *elasticsearchSwitch == "enable" {
		//初始化elasticsearch exporter
		http.HandleFunc("/elastic", handlers.NewElasticsearchHandler().ServeHTTP)
	}

	if *kafkaSwitch == "enable" {
		//初始化kafka exporter
		http.HandleFunc("/kafka", handlers.NewKafkaHandler().ServeHTTP)
	}

	if *postgresSwitch == "enable" {
		//初始化postgres exporter
		http.HandleFunc("/postgres", handlers.NewPostgresHandler().ServeHTTP)
	}

	http.HandleFunc("/node", handlers.NodeExporterHandler)

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

func initNodeExporter () {
	// 初始化node collectors
	nc, err := collector.NewNodeCollector()
	if err != nil {
		log.Fatalf("Couldn't create collector: %s", err)
	}
	log.Infof("Enabled node collectors:")
	collectors := []string{}
	for n := range nc.Collectors {
		collectors = append(collectors, n)
	}
	sort.Strings(collectors)
	for _, n := range collectors {
		log.Infof(" - %s", n)
	}

}

func getMysqlExporterScrapers() map[mysqldcollector.Scraper]*bool {
	scraperFlags := map[mysqldcollector.Scraper]*bool{}
	for scraper, enabledByDefault := range handlers.Scrapers {
		defaultOn := "false"
		if enabledByDefault {
			defaultOn = "true"
		}

		f := kingpin.Flag(
			"mysqld."+scraper.Name(),
			scraper.Help(),
		).Default(defaultOn).Bool()
		scraperFlags[scraper] = f
	}
	return scraperFlags

}

func parseFlags() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("exporter-plus"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
}

func initMysqlExporter (scrapers map[mysqldcollector.Scraper]*bool) []mysqldcollector.Scraper {
	if *handlers.Dsn == "" {
		log.Fatal(fmt.Errorf("data source name could not be null"))
	}

	// Register only scrapers enabled by flag.
	log.Infof("Enabled mysqld collectors:")
	enabledScrapers := []mysqldcollector.Scraper{}
	for scraper, enabled := range scrapers {
		if *enabled {
			log.Infof(" --collect.%s", scraper.Name())
			enabledScrapers = append(enabledScrapers, scraper)
		}
	}
	return enabledScrapers
}
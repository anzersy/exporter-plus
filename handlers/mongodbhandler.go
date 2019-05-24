package handlers

import (
	"exporter-plus/collector/mongodb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
	"net/http"
)

var (
	mongodbURIFlag = kingpin.Flag(
		"mongodb.uri",
		"Mongodb URI, format: [mongodb://][user:pass@]host1[:port1][,host2[:port2],...][/database][?options]",
	).Default("mongodb://localhost:27017").String()

	mongodbTLSCert = kingpin.Flag(
		"mongodb.tls-cert",
		"Path to PEM file that conains the certificate (and optionally also the private key in PEM format).\n"+
			"    \tThis should include the whole certificate chain.\n"+
			"    \tIf provided: The connection will be opened via TLS to the MongoDB server.",
	).Default("").String()

	mongodbTLSPrivateKey = kingpin.Flag(
		"mongodb.tls-private-key",
		"Path to PEM file that conains the private key (if not contained in mongodb.tls-cert file).",
	).Default("").String()

	mongodbTLSCa = kingpin.Flag(
		"mongodb.tls-ca",
		"Path to PEM file that conains the CAs that are trused for server connections.\n"+
			"    \tIf provided: MongoDB servers connecting to should present a certificate signed by one of this CAs.\n"+
			"    \tIf not provided: System default CAs are used.",
	).Default("").String()

	mongodbUserName = kingpin.Flag(
		"mongodb.username",
		"Username to connect to Mongodb",
	).Default("").String()

	mongodbAuthMechanism = kingpin.Flag(
		"mongodb.mechanism",
		"auth mechanism to connect to Mongodb (ie: MONGODB-X509)",
	).Default("").String()

	mongodbTLSDisableHostnameValidation = kingpin.Flag(
		"mongodb.tls-disable-hostname-validation",
		"Do hostname validation for server connection.",
	).Default("false").Bool()

	mongodbCollectOplog = kingpin.Flag(
		"mongodb.collect.oplog",
		"collect Mongodb Oplog status",
	).Default("true").Bool()

	mongodbCollectOplogTail = kingpin.Flag(
		"mongodb.collect.oplog_tail",
		"tail Mongodb Oplog to get stats",
	).Default("false").Bool()

	mongodbCollectReplSet = kingpin.Flag(
		"mongodb.collect.replset",
		"collect Mongodb replica set status",
	).Default("true").Bool()

	mongodbCollectTopMetrics = kingpin.Flag(
		"mongodb.collect.top",
		"collect Mongodb Top metrics",
	).Default("false").Bool()

	mongodbCollectDatabaseMetrics = kingpin.Flag(
		"mongodb.collect.database",
		"collect MongoDB database metrics",
	).Default("false").Bool()

	mongodbCollectCollectionMetrics = kingpin.Flag(
		"mongodb.collect.collection",
		"Collect MongoDB collection metrics",
	).Default("false").Bool()

	mongodbCollectProfileMetrics = kingpin.Flag(
		"mongodb.collect.profile",
		"Collect MongoDB profile metrics",
	).Default("false").Bool()

	mongodbCollectConnPoolStats = kingpin.Flag(
		"mongodb.collect.connpoolstats",
		"Collect MongoDB connpoolstats",
	).Default("false").Bool()

	mongodbSocketTimeout = kingpin.Flag(
		"mongodb.socket-timeout",
		"timeout for socket operations to mongodb",
	).Default("0").Duration()
)

func NewMongodbHandler() http.Handler {

	mongodbCollector := mongodb.NewMongodbCollector(mongodb.MongodbCollectorOpts{
		URI:                      *mongodbURIFlag,
		TLSCertificateFile:       *mongodbTLSCert,
		TLSPrivateKeyFile:        *mongodbTLSPrivateKey,
		TLSCaFile:                *mongodbTLSCa,
		TLSHostnameValidation:    !(*mongodbTLSDisableHostnameValidation),
		CollectOplog:             *mongodbCollectOplog,
		TailOplog:                *mongodbCollectOplogTail,
		CollectReplSet:           *mongodbCollectReplSet,
		CollectTopMetrics:        *mongodbCollectTopMetrics,
		CollectDatabaseMetrics:   *mongodbCollectDatabaseMetrics,
		CollectCollectionMetrics: *mongodbCollectCollectionMetrics,
		CollectProfileMetrics:    *mongodbCollectProfileMetrics,
		CollectConnPoolStats:     *mongodbCollectConnPoolStats,
		UserName:                 *mongodbUserName,
		AuthMechanism:            *mongodbAuthMechanism,
		SocketTimeout:            *mongodbSocketTimeout,
	})

	registry := prometheus.NewRegistry()
	registry.MustRegister(mongodbCollector)
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return handler

}

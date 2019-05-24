package handlers

import (
	"github.com/Shopify/sarama"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/alecthomas/kingpin.v2"
	"exporter-plus/collector/kafka"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	topicFilter   = kingpin.Flag("topic.filter", "Regex that determines which topics to collect.").Default(".*").String()
	groupFilter   = kingpin.Flag("group.filter", "Regex that determines which consumer groups to collect.").Default(".*").String()
	logSarama     = kingpin.Flag("log.enable-sarama", "Turn on Sarama logging.").Default("false").Bool()

	opts = kafka.KafkaOpts{}
)

func GetKafkaFlag(){
	kingpin.Flag("kafka.server", "Address (host:port) of Kafka server.").Default("kafka:9092").StringsVar(&opts.Uri)
	kingpin.Flag("sasl.enabled", "Connect using SASL/PLAIN.").Default("false").BoolVar(&opts.UseSASL)
	kingpin.Flag("sasl.handshake", "Only set this to false if using a non-Kafka SASL proxy.").Default("true").BoolVar(&opts.UseSASLHandshake)
	kingpin.Flag("sasl.username", "SASL user name.").Default("").StringVar(&opts.SaslUsername)
	kingpin.Flag("sasl.password", "SASL user password.").Default("").StringVar(&opts.SaslPassword)
	kingpin.Flag("tls.enabled", "Connect using TLS.").Default("false").BoolVar(&opts.UseTLS)
	kingpin.Flag("tls.ca-file", "The optional certificate authority file for TLS client authentication.").Default("").StringVar(&opts.TlsCAFile)
	kingpin.Flag("tls.cert-file", "The optional certificate file for client authentication.").Default("").StringVar(&opts.TlsCertFile)
	kingpin.Flag("tls.key-file", "The optional key file for client authentication.").Default("").StringVar(&opts.TlsKeyFile)
	kingpin.Flag("tls.insecure-skip-tls-verify", "If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure.").Default("false").BoolVar(&opts.TlsInsecureSkipTLSVerify)
	kingpin.Flag("kafka.version", "Kafka broker version").Default(sarama.V1_0_0_0.String()).StringVar(&opts.KafkaVersion)
	kingpin.Flag("use.consumelag.zookeeper", "if you need to use a group from zookeeper").Default("false").BoolVar(&opts.UseZooKeeperLag)
	kingpin.Flag("zookeeper.server", "Address (hosts) of zookeeper server.").Default("localhost:2181").StringsVar(&opts.UriZookeeper)
	kingpin.Flag("kafka.labels", "Kafka cluster name").Default("").StringVar(&opts.Labels)
	kingpin.Flag("refresh.metadata", "Metadata refresh interval").Default("30s").StringVar(&opts.MetadataRefreshInterval)

}


func NewKafkaHandler() http.Handler {
	labels := make(map[string]string)

	// Protect against empty labels
	if opts.Labels != "" {
		for _, label := range strings.Split(opts.Labels, ",") {
			splitted := strings.Split(label, "=")
			if len(splitted) >= 2 {
				labels[splitted[0]] = splitted[1]
			}
		}
	}

	kafka.ClusterBrokers = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "", "brokers"),
		"Number of Brokers in the Kafka Cluster.",
		nil, labels,
	)
	kafka.TopicPartitions = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "topic", "partitions"),
		"Number of partitions for this Topic",
		[]string{"topic"}, labels,
	)
	kafka.TopicCurrentOffset = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "topic", "partition_current_offset"),
		"Current Offset of a Broker at Topic/Partition",
		[]string{"topic", "partition"}, labels,
	)
	kafka.TopicOldestOffset = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "topic", "partition_oldest_offset"),
		"Oldest Offset of a Broker at Topic/Partition",
		[]string{"topic", "partition"}, labels,
	)

	kafka.TopicPartitionLeader = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "topic", "partition_leader"),
		"Leader Broker ID of this Topic/Partition",
		[]string{"topic", "partition"}, labels,
	)

	kafka.TopicPartitionReplicas = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "topic", "partition_replicas"),
		"Number of Replicas for this Topic/Partition",
		[]string{"topic", "partition"}, labels,
	)

	kafka.TopicPartitionInSyncReplicas = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "topic", "partition_in_sync_replica"),
		"Number of In-Sync Replicas for this Topic/Partition",
		[]string{"topic", "partition"}, labels,
	)

	kafka.TopicPartitionUsesPreferredReplica = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "topic", "partition_leader_is_preferred"),
		"1 if Topic/Partition is using the Preferred Broker",
		[]string{"topic", "partition"}, labels,
	)

	kafka.TopicUnderReplicatedPartition = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "topic", "partition_under_replicated_partition"),
		"1 if Topic/Partition is under Replicated",
		[]string{"topic", "partition"}, labels,
	)

	kafka.ConsumergroupCurrentOffset = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "consumergroup", "current_offset"),
		"Current Offset of a ConsumerGroup at Topic/Partition",
		[]string{"consumergroup", "topic", "partition"}, labels,
	)

	kafka.ConsumergroupCurrentOffsetSum = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "consumergroup", "current_offset_sum"),
		"Current Offset of a ConsumerGroup at Topic for all partitions",
		[]string{"consumergroup", "topic"}, labels,
	)

	kafka.ConsumergroupLag = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "consumergroup", "lag"),
		"Current Approximate Lag of a ConsumerGroup at Topic/Partition",
		[]string{"consumergroup", "topic", "partition"}, labels,
	)

	kafka.ConsumergroupLagZookeeper = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "consumergroupzookeeper", "lag_zookeeper"),
		"Current Approximate Lag(zookeeper) of a ConsumerGroup at Topic/Partition",
		[]string{"consumergroup", "topic", "partition"}, nil,
	)

	kafka.ConsumergroupLagSum = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "consumergroup", "lag_sum"),
		"Current Approximate Lag of a ConsumerGroup at Topic for all partitions",
		[]string{"consumergroup", "topic"}, labels,
	)

	kafka.ConsumergroupMembers = prometheus.NewDesc(
		prometheus.BuildFQName(kafka.Namespace, "consumergroup", "members"),
		"Amount of members in a consumer group",
		[]string{"consumergroup"}, labels,
	)

	if *logSarama {
		sarama.Logger = log.New(os.Stdout, "[sarama] ", log.LstdFlags)
	}


	exporter, err := kafka.NewExporter(opts, *topicFilter, *groupFilter)
	if err != nil {
		log.Fatalln(err)
	}
	//defer exporter.client.Close()

	registry := prometheus.NewRegistry()
	registry.MustRegister(exporter)

	gatherers := prometheus.Gatherers{
		//prometheus.DefaultGatherer,
		registry,
	}

	handler := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{})
	return handler
}
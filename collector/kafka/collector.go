package kafka

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	kazoo "github.com/krallistic/kazoo-go"
	"github.com/prometheus/client_golang/prometheus"
	plog "github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/rcrowley/go-metrics"
)

const (
	clientID  = "kafka_exporter"
)
var (
	Namespace = "kafka"
)

var (
	ClusterBrokers                     *prometheus.Desc
	TopicPartitions                    *prometheus.Desc
	TopicCurrentOffset                 *prometheus.Desc
	TopicOldestOffset                  *prometheus.Desc
	TopicPartitionLeader               *prometheus.Desc
	TopicPartitionReplicas             *prometheus.Desc
	TopicPartitionInSyncReplicas       *prometheus.Desc
	TopicPartitionUsesPreferredReplica *prometheus.Desc
	TopicUnderReplicatedPartition      *prometheus.Desc
	ConsumergroupCurrentOffset         *prometheus.Desc
	ConsumergroupCurrentOffsetSum      *prometheus.Desc
	ConsumergroupLag                   *prometheus.Desc
	ConsumergroupLagSum                *prometheus.Desc
	ConsumergroupLagZookeeper          *prometheus.Desc
	ConsumergroupMembers               *prometheus.Desc
)

// Exporter collects Kafka stats from the given server and exports them using
// the prometheus metrics package.
type Exporter struct {
	Client                  sarama.Client
	topicFilter             *regexp.Regexp
	groupFilter             *regexp.Regexp
	mu                      sync.Mutex
	useZooKeeperLag         bool
	zookeeperClient         *kazoo.Kazoo
	nextMetadataRefresh     time.Time
	metadataRefreshInterval time.Duration
}

type KafkaOpts struct {
	Uri                      []string
	UseSASL                  bool
	UseSASLHandshake         bool
	SaslUsername             string
	SaslPassword             string
	UseTLS                   bool
	TlsCAFile                string
	TlsCertFile              string
	TlsKeyFile               string
	TlsInsecureSkipTLSVerify bool
	KafkaVersion             string
	UseZooKeeperLag          bool
	UriZookeeper             []string
	Labels                   string
	MetadataRefreshInterval  string
}

// CanReadCertAndKey returns true if the certificate and key files already exists,
// otherwise returns false. If lost one of cert and key, returns error.
func CanReadCertAndKey(certPath, keyPath string) (bool, error) {
	certReadable := canReadFile(certPath)
	keyReadable := canReadFile(keyPath)

	if certReadable == false && keyReadable == false {
		return false, nil
	}

	if certReadable == false {
		return false, fmt.Errorf("error reading %s, certificate and key must be supplied as a pair", certPath)
	}

	if keyReadable == false {
		return false, fmt.Errorf("error reading %s, certificate and key must be supplied as a pair", keyPath)
	}

	return true, nil
}

// If the file represented by path exists and
// readable, returns true otherwise returns false.
func canReadFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}

	defer f.Close()

	return true
}

// NewExporter returns an initialized Exporter.
func NewExporter(opts KafkaOpts, topicFilter string, groupFilter string) (*Exporter, error) {
	fmt.Println(opts)
	var zookeeperClient *kazoo.Kazoo
	config := sarama.NewConfig()
	config.ClientID = clientID
	kafkaVersion, err := sarama.ParseKafkaVersion(opts.KafkaVersion)
	if err != nil {
		return nil, err
	}
	config.Version = kafkaVersion

	if opts.UseSASL {
		config.Net.SASL.Enable = true
		config.Net.SASL.Handshake = opts.UseSASLHandshake

		if opts.SaslUsername != "" {
			config.Net.SASL.User = opts.SaslUsername
		}

		if opts.SaslPassword != "" {
			config.Net.SASL.Password = opts.SaslPassword
		}
	}

	if opts.UseTLS {
		config.Net.TLS.Enable = true

		config.Net.TLS.Config = &tls.Config{
			RootCAs:            x509.NewCertPool(),
			InsecureSkipVerify: opts.TlsInsecureSkipTLSVerify,
		}

		if opts.TlsCAFile != "" {
			if ca, err := ioutil.ReadFile(opts.TlsCAFile); err == nil {
				config.Net.TLS.Config.RootCAs.AppendCertsFromPEM(ca)
			} else {
				plog.Fatalln(err)
			}
		}

		canReadCertAndKey, err := CanReadCertAndKey(opts.TlsCertFile, opts.TlsKeyFile)
		if err != nil {
			plog.Fatalln(err)
		}
		if canReadCertAndKey {
			cert, err := tls.LoadX509KeyPair(opts.TlsCertFile, opts.TlsKeyFile)
			if err == nil {
				config.Net.TLS.Config.Certificates = []tls.Certificate{cert}
			} else {
				plog.Fatalln(err)
			}
		}
	}

	if opts.UseZooKeeperLag {
		zookeeperClient, err = kazoo.NewKazoo(opts.UriZookeeper, nil)
	}

	interval, err := time.ParseDuration(opts.MetadataRefreshInterval)
	if err != nil {
		plog.Errorln("Cannot parse metadata refresh interval")
		panic(err)
	}

	config.Metadata.RefreshFrequency = interval

	client, err := sarama.NewClient(opts.Uri, config)
	fmt.Println(config)
	a, _ := client.Topics()
	fmt.Println(a,"ssss")
	if err != nil {
		plog.Errorln("Error Init Kafka Client")
		panic(err)
	}
	plog.Infoln("Done Init Clients")

	// Init our exporter.
	return &Exporter{
		Client:                  client,
		topicFilter:             regexp.MustCompile(topicFilter),
		groupFilter:             regexp.MustCompile(groupFilter),
		useZooKeeperLag:         opts.UseZooKeeperLag,
		zookeeperClient:         zookeeperClient,
		nextMetadataRefresh:     time.Now(),
		metadataRefreshInterval: interval,
	}, nil
}

// Describe describes all the metrics ever exported by the Kafka exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- ClusterBrokers
	ch <- TopicCurrentOffset
	ch <- TopicOldestOffset
	ch <- TopicPartitions
	ch <- TopicPartitionLeader
	ch <- TopicPartitionReplicas
	ch <- TopicPartitionInSyncReplicas
	ch <- TopicPartitionUsesPreferredReplica
	ch <- TopicUnderReplicatedPartition
	ch <- ConsumergroupCurrentOffset
	ch <- ConsumergroupCurrentOffsetSum
	ch <- ConsumergroupLag
	ch <- ConsumergroupLagZookeeper
	ch <- ConsumergroupLagSum
}

// Collect fetches the stats from configured Kafka location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	var wg = sync.WaitGroup{}
	fmt.Println(len(e.Client.Brokers()))
	ch <- prometheus.MustNewConstMetric(
		ClusterBrokers, prometheus.GaugeValue, float64(len(e.Client.Brokers())),
	)

	offset := make(map[string]map[int32]int64)

	now := time.Now()

	if now.After(e.nextMetadataRefresh) {
		plog.Info("Refreshing client metadata")

		if err := e.Client.RefreshMetadata(); err != nil {
			plog.Errorf("Cannot refresh topics, using cached data: %v", err)
		}

		e.nextMetadataRefresh = now.Add(e.metadataRefreshInterval)
	}

	topics, err := e.Client.Topics()
	fmt.Println(topics)
	if err != nil {
		plog.Errorf("Cannot get topics: %v", err)
		return
	}

	getTopicMetrics := func(topic string) {
		defer wg.Done()
		if e.topicFilter.MatchString(topic) {
			partitions, err := e.Client.Partitions(topic)
			if err != nil {
				plog.Errorf("Cannot get partitions of topic %s: %v", topic, err)
				return
			}
			ch <- prometheus.MustNewConstMetric(
				TopicPartitions, prometheus.GaugeValue, float64(len(partitions)), topic,
			)
			e.mu.Lock()
			offset[topic] = make(map[int32]int64, len(partitions))
			e.mu.Unlock()
			for _, partition := range partitions {
				broker, err := e.Client.Leader(topic, partition)
				if err != nil {
					plog.Errorf("Cannot get leader of topic %s partition %d: %v", topic, partition, err)
				} else {
					ch <- prometheus.MustNewConstMetric(
						TopicPartitionLeader, prometheus.GaugeValue, float64(broker.ID()), topic, strconv.FormatInt(int64(partition), 10),
					)
				}

				currentOffset, err := e.Client.GetOffset(topic, partition, sarama.OffsetNewest)
				if err != nil {
					plog.Errorf("Cannot get current offset of topic %s partition %d: %v", topic, partition, err)
				} else {
					e.mu.Lock()
					offset[topic][partition] = currentOffset
					e.mu.Unlock()
					ch <- prometheus.MustNewConstMetric(
						TopicCurrentOffset, prometheus.GaugeValue, float64(currentOffset), topic, strconv.FormatInt(int64(partition), 10),
					)
				}

				oldestOffset, err := e.Client.GetOffset(topic, partition, sarama.OffsetOldest)
				if err != nil {
					plog.Errorf("Cannot get oldest offset of topic %s partition %d: %v", topic, partition, err)
				} else {
					ch <- prometheus.MustNewConstMetric(
						TopicOldestOffset, prometheus.GaugeValue, float64(oldestOffset), topic, strconv.FormatInt(int64(partition), 10),
					)
				}

				replicas, err := e.Client.Replicas(topic, partition)
				if err != nil {
					plog.Errorf("Cannot get replicas of topic %s partition %d: %v", topic, partition, err)
				} else {
					ch <- prometheus.MustNewConstMetric(
						TopicPartitionReplicas, prometheus.GaugeValue, float64(len(replicas)), topic, strconv.FormatInt(int64(partition), 10),
					)
				}

				inSyncReplicas, err := e.Client.InSyncReplicas(topic, partition)
				if err != nil {
					plog.Errorf("Cannot get in-sync replicas of topic %s partition %d: %v", topic, partition, err)
				} else {
					ch <- prometheus.MustNewConstMetric(
						TopicPartitionInSyncReplicas, prometheus.GaugeValue, float64(len(inSyncReplicas)), topic, strconv.FormatInt(int64(partition), 10),
					)
				}

				if broker != nil && replicas != nil && len(replicas) > 0 && broker.ID() == replicas[0] {
					ch <- prometheus.MustNewConstMetric(
						TopicPartitionUsesPreferredReplica, prometheus.GaugeValue, float64(1), topic, strconv.FormatInt(int64(partition), 10),
					)
				} else {
					ch <- prometheus.MustNewConstMetric(
						TopicPartitionUsesPreferredReplica, prometheus.GaugeValue, float64(0), topic, strconv.FormatInt(int64(partition), 10),
					)
				}

				if replicas != nil && inSyncReplicas != nil && len(inSyncReplicas) < len(replicas) {
					ch <- prometheus.MustNewConstMetric(
						TopicUnderReplicatedPartition, prometheus.GaugeValue, float64(1), topic, strconv.FormatInt(int64(partition), 10),
					)
				} else {
					ch <- prometheus.MustNewConstMetric(
						TopicUnderReplicatedPartition, prometheus.GaugeValue, float64(0), topic, strconv.FormatInt(int64(partition), 10),
					)
				}

				if e.useZooKeeperLag {
					ConsumerGroups, err := e.zookeeperClient.Consumergroups()

					if err != nil {
						plog.Errorf("Cannot get consumer group %v", err)
					}

					for _, group := range ConsumerGroups {
						offset, _ := group.FetchOffset(topic, partition)
						if offset > 0 {

							consumerGroupLag := currentOffset - offset
							ch <- prometheus.MustNewConstMetric(
								ConsumergroupLagZookeeper, prometheus.GaugeValue, float64(consumerGroupLag), group.Name, topic, strconv.FormatInt(int64(partition), 10),
							)
						}
					}
				}
			}
		}
	}

	for _, topic := range topics {
		wg.Add(1)
		go getTopicMetrics(topic)
	}

	wg.Wait()

	getConsumerGroupMetrics := func(broker *sarama.Broker) {
		defer wg.Done()
		if err := broker.Open(e.Client.Config()); err != nil && err != sarama.ErrAlreadyConnected {
			plog.Errorf("Cannot connect to broker %d: %v", broker.ID(), err)
			return
		}
		defer broker.Close()

		groups, err := broker.ListGroups(&sarama.ListGroupsRequest{})
		if err != nil {
			plog.Errorf("Cannot get consumer group: %v", err)
			return
		}
		groupIds := make([]string, 0)
		for groupId := range groups.Groups {
			if e.groupFilter.MatchString(groupId) {
				groupIds = append(groupIds, groupId)
			}
		}

		fmt.Println(groupIds)

		describeGroups, err := broker.DescribeGroups(&sarama.DescribeGroupsRequest{Groups: groupIds})
		if err != nil {
			plog.Errorf("Cannot get describe groups: %v", err)
			return
		}

		fmt.Println(describeGroups)
		for _, group := range describeGroups.Groups {
			offsetFetchRequest := sarama.OffsetFetchRequest{ConsumerGroup: group.GroupId, Version: 1}
			for topic, partitions := range offset {
				for partition := range partitions {
					offsetFetchRequest.AddPartition(topic, partition)
				}
			}
			ch <- prometheus.MustNewConstMetric(
				ConsumergroupMembers, prometheus.GaugeValue, float64(len(group.Members)), group.GroupId,
			)
			if offsetFetchResponse, err := broker.FetchOffset(&offsetFetchRequest); err != nil {
				plog.Errorf("Cannot get offset of group %s: %v", group.GroupId, err)
			} else {
				for topic, partitions := range offsetFetchResponse.Blocks {
					// If the topic is not consumed by that consumer group, skip it
					topicConsumed := false
					for _, offsetFetchResponseBlock := range partitions {
						// Kafka will return -1 if there is no offset associated with a topic-partition under that consumer group
						if offsetFetchResponseBlock.Offset != -1 {
							topicConsumed = true
							break
						}
					}
					if topicConsumed {
						var currentOffsetSum int64
						var lagSum int64
						for partition, offsetFetchResponseBlock := range partitions {
							err := offsetFetchResponseBlock.Err
							if err != sarama.ErrNoError {
								plog.Errorf("Error for  partition %d :%v", partition, err.Error())
								continue
							}
							currentOffset := offsetFetchResponseBlock.Offset
							currentOffsetSum += currentOffset
							ch <- prometheus.MustNewConstMetric(
								ConsumergroupCurrentOffset, prometheus.GaugeValue, float64(currentOffset), group.GroupId, topic, strconv.FormatInt(int64(partition), 10),
							)
							e.mu.Lock()
							if offset, ok := offset[topic][partition]; ok {
								// If the topic is consumed by that consumer group, but no offset associated with the partition
								// forcing lag to -1 to be able to alert on that
								var lag int64
								if offsetFetchResponseBlock.Offset == -1 {
									lag = -1
								} else {
									lag = offset - offsetFetchResponseBlock.Offset
									lagSum += lag
								}
								ch <- prometheus.MustNewConstMetric(
									ConsumergroupLag, prometheus.GaugeValue, float64(lag), group.GroupId, topic, strconv.FormatInt(int64(partition), 10),
								)
							} else {
								plog.Errorf("No offset of topic %s partition %d, cannot get consumer group lag", topic, partition)
							}
							e.mu.Unlock()
						}
						ch <- prometheus.MustNewConstMetric(
							ConsumergroupCurrentOffsetSum, prometheus.GaugeValue, float64(currentOffsetSum), group.GroupId, topic,
						)
						ch <- prometheus.MustNewConstMetric(
							ConsumergroupLagSum, prometheus.GaugeValue, float64(lagSum), group.GroupId, topic,
						)
					}
				}
			}
		}
	}

	if len(e.Client.Brokers()) > 0 {
		for _, broker := range e.Client.Brokers() {
			wg.Add(1)
			go getConsumerGroupMetrics(broker)
		}
		wg.Wait()
	} else {
		plog.Errorln("No valid broker, cannot get consumer group metrics")
	}
}

func init() {
	metrics.UseNilMetrics = true
	prometheus.MustRegister(version.NewCollector("kafka_exporter"))
}
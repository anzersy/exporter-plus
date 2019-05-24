package handlers

import (
	"github.com/oliver006/redis_exporter/exporter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
	"io/ioutil"
	"net/http"
)

var (
	redisAddr = kingpin.Flag(
		"redis.addr",
		"Address of one or more redis nodes, separated by separator",
	).Default("0.0.0.0:6379").String()

	redisFile = kingpin.Flag(
		"redis.file",
		"Path to file containing one or more redis nodes, separated by newline. NOTE: mutually exclusive with redis.addr",
	).Default("").String()

	redisPassword = kingpin.Flag(
		"redis.password",
		"Password for one or more redis nodes, separated by separator",
	).Default("").String()

	redisPasswordFile = kingpin.Flag(
		"redis.password-file",
		"File containing the password for one or more redis nodes, separated by separator. NOTE: mutually exclusive with redis.password",
	).Default("").String()

	redisAlias = kingpin.Flag(
		"redis.alias",
		"Redis instance alias for one or more redis nodes, separated by separator",
	).Default("").String()

	namespace = kingpin.Flag(
		"redis.namespace",
		"Namespace for metrics",
	).Default("").String()

	checkKeys = kingpin.Flag(
		"redis.check-keys",
		"Comma separated list of key-patterns to export value and length/size, searched for with SCAN",
	).Default("").String()

	checkSingleKeys = kingpin.Flag(
		"redis.check-single-keys",
		"Comma separated list of single keys to export value and length/size",
	).Default("").String()

	scriptPath = kingpin.Flag(
		"redis.script",
		"Path to Lua Redis script for collecting extra metrics",
	).Default("").String()

	separator = kingpin.Flag(
		"redis.separator",
		"separator used to split redis.addr, redis.password and redis.alias into several elements.",
	).Default("").String()

	useCfBindings = kingpin.Flag(
		"redis.use-cf-bindings",
		"Use Cloud Foundry service bindings",
	).Default("false").Bool()

)

func NewRedisHandler() http.Handler {

	if *redisFile != "" && *redisAddr != "" {
		log.Fatal("Cannot specify both redis.addr and redis.file")
	}

	var parsedRedisPassword string

	if *redisPasswordFile != "" {
		if *redisPassword != "" {
			log.Fatal("Cannot specify both redis.password and redis.password-file")
		}
		b, err := ioutil.ReadFile(*redisPasswordFile)
		if err != nil {
			log.Fatal(err)
		}
		parsedRedisPassword = string(b)
	} else {
		parsedRedisPassword = *redisPassword
	}

	var addrs, passwords, aliases []string

	switch {
	case *redisFile != "":
		var err error
		if addrs, passwords, aliases, err = exporter.LoadRedisFile(*redisFile); err != nil {
			log.Fatal(err)
		}
	case *useCfBindings:
		addrs, passwords, aliases = exporter.GetCloudFoundryRedisBindings()
	default:
		addrs, passwords, aliases = exporter.LoadRedisArgs(*redisAddr, parsedRedisPassword, *redisAlias, *separator)
	}

	exp, err := exporter.NewRedisExporter(
		exporter.RedisHost{Addrs: addrs, Passwords: passwords, Aliases: aliases},
		*namespace,
		*checkSingleKeys,
		*checkKeys,
	)
	if err != nil {
		log.Fatal(err)
	}

	if *scriptPath != "" {
		if exp.LuaScript, err = ioutil.ReadFile(*scriptPath); err != nil {
			log.Fatalf("Error loading script file %s    err: %s", *scriptPath, err)
		}
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(exp)
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	return handler

}

package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	oracleclient "github.com/facundomedica/connect-client"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rollkit/centralized-sequencer/sequencing"
	sequencingGRPC "github.com/rollkit/go-sequencing/proxy/grpc"
	oracleconfig "github.com/skip-mev/connect/v2/oracle/config"
	oracletypes "github.com/skip-mev/connect/v2/service/servers/oracle/types"

	sdklog "cosmossdk.io/log"
	"github.com/facundomedica/rollinky/sequencer/utils"
	"github.com/skip-mev/connect/v2/service/metrics"
)

const (
	defaultHost      = "localhost"
	defaultPort      = "50051"
	defaultBatchTime = 2 * time.Second
	defaultDA        = "http://localhost:26658"
)

func main() {
	var (
		host             string
		port             string
		listenAll        bool
		rollupId         string
		batchTime        time.Duration
		da_address       string
		da_namespace     string
		da_auth_token    string
		db_path          string
		metricsEnabled   bool
		metricsAddress   string
		signerID         string
		oracleConfigPath string
	)
	flag.StringVar(&host, "host", defaultHost, "centralized sequencer host")
	flag.StringVar(&port, "port", defaultPort, "centralized sequencer port")
	flag.BoolVar(&listenAll, "listen-all", false, "listen on all network interfaces (0.0.0.0) instead of just localhost")
	flag.StringVar(&rollupId, "rollup-id", "rollupId", "rollup id")
	flag.DurationVar(&batchTime, "batch-time", defaultBatchTime, "time in seconds to wait before generating a new batch")
	flag.StringVar(&da_address, "da_address", defaultDA, "DA address")
	flag.StringVar(&da_namespace, "da_namespace", "", "DA namespace where the sequencer submits transactions")
	flag.StringVar(&da_auth_token, "da_auth_token", "", "auth token for the DA")
	flag.StringVar(&db_path, "db_path", "", "path to the database")
	flag.BoolVar(&metricsEnabled, "metrics", false, "Enable Prometheus metrics")
	flag.StringVar(&metricsAddress, "metrics-address", ":8080", "Address to expose Prometheus metrics")
	flag.StringVar(&signerID, "signer-id", "", "Intel SGX signer ID")
	flag.StringVar(&oracleConfigPath, "config", "config.toml", "path to oracle config file")

	flag.Parse()

	if listenAll {
		host = "0.0.0.0"
	}

	address := fmt.Sprintf("%s:%s", host, port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	namespace := make([]byte, len(da_namespace)/2)
	_, err = hex.Decode(namespace, []byte(da_namespace))
	if err != nil {
		log.Fatalf("Error decoding namespace: %v", err)
	}

	var metricsServer *http.Server
	if metricsEnabled {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		metricsServer = &http.Server{
			Addr:              metricsAddress,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			log.Printf("Starting metrics server on %v...\n", metricsAddress)
			if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
				log.Fatalf("Failed to serve metrics: %v", err)
			}
		}()
	}

	metrics, err := sequencing.DefaultMetricsProvider(metricsEnabled)(da_namespace)
	if err != nil {
		log.Fatalf("Failed to create metrics: %v", err)
	}

	oracleCfg := oracleconfig.NewDefaultAppConfig()
	if _, err := toml.DecodeFile(oracleConfigPath, &oracleCfg); err != nil {
		log.Fatalf("Failed to decode config file: %v", err)
	}
	oracle := NewOracle(oracleCfg, signerID)

	centralizedSeq, err := sequencing.NewSequencer(da_address, da_auth_token, namespace, []byte(rollupId), batchTime, metrics, db_path, oracle)
	if err != nil {
		log.Fatalf("Failed to create centralized sequencer: %v", err)
	}
	grpcServer := sequencingGRPC.NewServer(centralizedSeq, centralizedSeq, centralizedSeq)

	log.Println("Starting centralized sequencing gRPC server on port 50051...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGINT)
	<-interrupt
	if metricsServer != nil {
		if err := metricsServer.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down metrics server: %v", err)
		}
	}
	fmt.Println("\nCtrl+C pressed. Exiting...")
	os.Exit(0)
}

type Oracle struct {
	oracleClient oracleclient.OracleClient
	signerID     []byte
}

func NewOracle(oracleCfg oracleconfig.AppConfig, signerID string) *Oracle {
	oracle := &Oracle{}

	var err error
	oracle.signerID, err = hex.DecodeString(signerID)
	if err != nil {
		panic(err)
	}

	oracle.oracleClient, err = oracleclient.NewPriceDaemonClientFromConfig(
		oracleCfg,
		sdklog.NewLogger(os.Stderr),
		metrics.NewMetrics("rollinky"),
	)
	if err != nil {
		panic(err)
	}

	go func() {
		err = oracle.oracleClient.Start(context.Background())
		if err != nil {
			panic(err)
		}
	}()

	return oracle
}

var _ sequencing.BatchExtender = (*Oracle)(nil)

// Head implements sequencing.BatchExtender.
func (o *Oracle) Head(max uint64) ([]byte, error) {
	if cc, ok := o.oracleClient.(oracleclient.WithTrailer); ok {
		start := time.Now()
		ctx, _ := context.WithTimeout(context.Background(), time.Second*3)
		prices, trailer, err := cc.PricesWithTrailer(ctx, &oracletypes.QueryPricesRequest{})

		if prices == nil || err != nil {
			fmt.Println("prices is nil")
			return nil, nil
		}

		pricesBz, err := prices.Marshal()
		if err != nil {
			fmt.Println("Error marshalling prices: ", err)
			return nil, err
		}

		fullTrailer := trailer.Get("x-enclave-report")
		if len(fullTrailer) == 0 {
			fmt.Println("Trailer is empty")
			return nil, nil
		}

		report := fullTrailer[0]

		enclaveReport, err := base64.RawStdEncoding.DecodeString(report)
		if err != nil {
			panic(err)
		}

		if err := utils.VerifyReport(enclaveReport, pricesBz, o.signerID); err != nil {
			panic(err)
		}

		fmt.Println("Verified prices!: ", prices.Prices, "That took: ", time.Since(start))

		return utils.Encode(pricesBz, enclaveReport), nil
	} else {
		fmt.Println("Oracle client does not support trailers")
	}

	return nil, nil
}

// Tail implements sequencing.BatchExtender.
func (o *Oracle) Tail(max uint64) ([]byte, error) {
	return nil, nil
}

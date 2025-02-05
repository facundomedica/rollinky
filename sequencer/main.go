package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/edgelesssys/ego/attestation"
	"github.com/edgelesssys/ego/attestation/tcbstatus"
	"github.com/edgelesssys/ego/eclient"
	oracleclient "github.com/facundomedica/connect-client"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rollkit/centralized-sequencer/sequencing"
	sequencingGRPC "github.com/rollkit/go-sequencing/proxy/grpc"
	oracleconfig "github.com/skip-mev/connect/v2/oracle/config"
	oracletypes "github.com/skip-mev/connect/v2/service/servers/oracle/types"

	sdklog "cosmossdk.io/log"
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
		host           string
		port           string
		listenAll      bool
		rollupId       string
		batchTime      time.Duration
		da_address     string
		da_namespace   string
		da_auth_token  string
		db_path        string
		metricsEnabled bool
		metricsAddress string
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

	oracle := NewOracle()

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
}

func NewOracle() *Oracle {
	oracleconfig := oracleconfig.NewDefaultAppConfig()
	oracleconfig.Enabled = true
	oracleconfig.OracleAddress = "20.4.69.13:8080"

	oracle := &Oracle{}
	var err error

	oracle.oracleClient, err = oracleclient.NewPriceDaemonClientFromConfig(
		oracleconfig,
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

		// TODO: we might need to error here in order to avoid creating blocks with no price
		if prices == nil || err != nil {
			fmt.Println("prices is nil")
			return nil, nil
		}

		pricesBz, err := prices.Marshal()
		if err != nil {
			fmt.Println("Error marshalling prices: ", err)
			return nil, err
		}

		// add to the first place
		// req.Txs = append([][]byte{pricesBz}, req.Txs...)
		report := trailer.Get("x-enclave-report")[0]

		enclaveReport, err := base64.RawStdEncoding.DecodeString(report)
		if err != nil {
			panic(err)
		}

		signer, err := hex.DecodeString("36d6f8cd12953b56d764ea4ce9fcff4526ae150c580cc8026b2ec9bb106d131e")
		if err != nil {
			panic(err)
		}

		if err := verifyReport(enclaveReport, pricesBz, signer); err != nil {
			panic(err)
		}

		fmt.Println("Verified prices!: ", prices.Prices, "That took: ", time.Since(start))
		// TODO: missing adding the report to the batch
		return pricesBz, nil
	} else {
		fmt.Println("Oracle client does not support trailers")
	}

	return nil, nil
}

// Tail implements sequencing.BatchExtender.
func (o *Oracle) Tail(max uint64) ([]byte, error) {
	return nil, nil
}

func verifyReport(reportBytes, certBytes, signer []byte) error {
	start := time.Now()
	report, err := eclient.VerifyRemoteReport(reportBytes)
	if err == attestation.ErrTCBLevelInvalid {
		fmt.Printf("Warning: TCB level is invalid: %v\n%v\n", report.TCBStatus, tcbstatus.Explain(report.TCBStatus))
		fmt.Println("We'll ignore this issue in this sample. For an app that should run in production, you must decide which of the different TCBStatus values are acceptable for you to continue.")
	} else if err != nil {
		return err
	}

	hash := sha256.Sum256(certBytes)
	if !bytes.Equal(report.Data[:len(hash)], hash[:]) {
		return errors.New("report data does not match the certificate's hash")
	}

	// You can either verify the UniqueID or the tuple (SignerID, ProductID, SecurityVersion, Debug).

	if report.SecurityVersion < 1 {
		return errors.New("invalid security version")
	}
	if binary.LittleEndian.Uint16(report.ProductID) != 1 {
		return errors.New("invalid product")
	}
	if !bytes.Equal(report.SignerID, signer) {
		return errors.New("invalid signer")
	}

	fmt.Println("Verification took:", time.Since(start))
	// For production, you must also verify that report.Debug == false

	return nil
}

package main

import (
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/buildbarn/bb-event-service/pkg/configuration"
	"github.com/buildbarn/bb-storage/pkg/ac"
	blobstore "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	build "google.golang.org/genproto/googleapis/devtools/build/v1"

	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: bb-event-service bb-event-service.conf")
	}

	eventServiceConfiguration, err := configuration.GetEventServiceConfiguration(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to read configuration from %s: %s", os.Args[1], err)
	}

	// Storage access.
	contentAddressableStorage, actionCache, err := blobstore.CreateBlobAccessObjectsFromConfig(eventServiceConfiguration.Blobstore)
	if err != nil {
		log.Fatal("Failed to create blob access: ", err)
	}

	// Web server for metrics and profiling.
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Fatal(http.ListenAndServe(eventServiceConfiguration.MetricsListenAddress, nil))
	}()

	// RPC server.
	s := grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	)
	build.RegisterPublishBuildEventServer(s, &buildEventServer{
		instanceName:              "bb-event-service",
		contentAddressableStorage: contentAddressableStorage,
		actionCache:               ac.NewBlobAccessActionCache(actionCache),

		streams: map[string]*streamState{},
	})
	grpc_prometheus.EnableHandlingTimeHistogram()
	grpc_prometheus.Register(s)
	sock, err := net.Listen("tcp", eventServiceConfiguration.GrpcListenAddress)
	if err != nil {
		log.Fatal("Failed to create listening socket: ", err)
	}
	if err := s.Serve(sock); err != nil {
		log.Fatal("Failed to serve RPC server: ", err)
	}
}

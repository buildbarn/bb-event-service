package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"

	"github.com/buildbarn/bb-storage/pkg/ac"
	"github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	build "google.golang.org/genproto/googleapis/devtools/build/v1"

	"google.golang.org/grpc"
)

func main() {
	var (
		blobstoreConfig  = flag.String("blobstore-config", "/config/blobstore.conf", "Configuration for blob storage")
		webListenAddress = flag.String("web.listen-address", ":80", "Port on which to expose metrics")
	)
	flag.Parse()

	// Storage access.
	contentAddressableStorage, actionCache, err := configuration.CreateBlobAccessObjectsFromConfig(*blobstoreConfig)
	if err != nil {
		log.Fatal("Failed to create blob access: ", err)
	}

	// Web server for metrics and profiling.
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Fatal(http.ListenAndServe(*webListenAddress, nil))
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
	sock, err := net.Listen("tcp", ":8983")
	if err != nil {
		log.Fatal("Failed to create listening socket: ", err)
	}
	if err := s.Serve(sock); err != nil {
		log.Fatal("Failed to serve RPC server: ", err)
	}
}

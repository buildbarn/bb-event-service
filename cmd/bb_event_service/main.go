package main

import (
	"log"
	"net/http"
	"os"

	"github.com/buildbarn/bb-event-service/pkg/proto/configuration/bb_event_service"
	blobstore "github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	bb_grpc "github.com/buildbarn/bb-storage/pkg/grpc"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/gorilla/mux"
	build "google.golang.org/genproto/googleapis/devtools/build/v1"

	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: bb_event_service bb_event_service.jsonnet")
	}
	var configuration bb_event_service.ApplicationConfiguration
	if err := util.UnmarshalConfigurationFromFile(os.Args[1], &configuration); err != nil {
		log.Fatalf("Failed to read configuration from %s: %s", os.Args[1], err)
	}

	// Storage access.
	contentAddressableStorage, actionCache, err := blobstore.CreateBlobAccessObjectsFromConfig(
		configuration.Blobstore,
		int(configuration.MaximumMessageSizeBytes))
	if err != nil {
		log.Fatal("Failed to create blob access: ", err)
	}

	go func() {
		log.Fatal(
			"gRPC server failure: ",
			bb_grpc.NewGRPCServersFromConfigurationAndServe(
				configuration.GrpcServers,
				func(s *grpc.Server) {
					build.RegisterPublishBuildEventServer(s, &buildEventServer{
						instanceName:              "bb-event-service",
						contentAddressableStorage: contentAddressableStorage,
						actionCache:               actionCache,

						streams: map[string]*streamState{},
					})
				}))
	}()

	// Web server for metrics and profiling.
	router := mux.NewRouter()
	util.RegisterAdministrativeHTTPEndpoints(router)
	log.Fatal(http.ListenAndServe(configuration.HttpListenAddress, router))
}

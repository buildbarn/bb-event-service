package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	build "google.golang.org/genproto/googleapis/devtools/build/v1"

	"google.golang.org/grpc"
)

type noopBuildEventServer struct {
}

func (bes *noopBuildEventServer) PublishLifecycleEvent(ctx context.Context, in *build.PublishLifecycleEventRequest) (*empty.Empty, error) {
	log.Printf("PublishLifecycleEvent 1: %#v", in)
	log.Printf("PublishLifecycleEvent 2: %#v", in.BuildEvent.Event)
	return &empty.Empty{}, nil
}

func (bes *noopBuildEventServer) PublishBuildToolEventStream(stream build.PublishBuildEvent_PublishBuildToolEventStreamServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			log.Print(err)
			return err
		}
		log.Printf("PublishBuildToolEventStream 1: %#v", in)
		log.Printf("PublishBuildToolEventStream 2: %#v", in.OrderedBuildEvent.Event)

		if err := stream.Send(&build.PublishBuildToolEventStreamResponse{
			StreamId:       in.OrderedBuildEvent.StreamId,
			SequenceNumber: in.OrderedBuildEvent.SequenceNumber,
		}); err != nil {
			log.Print(err)
			return err
		}
	}
}

func main() {
	var (
		webListenAddress = flag.String("web.listen-address", ":80", "Port on which to expose metrics")
	)
	flag.Parse()

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
	build.RegisterPublishBuildEventServer(s, &noopBuildEventServer{})
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

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"sync"

	buildeventstream "github.com/bazelbuild/bazel/src/main/java/com/google/devtools/build/lib/buildeventstream/proto"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/buildbarn/bb-storage/pkg/ac"
	"github.com/buildbarn/bb-storage/pkg/blobstore"
	"github.com/buildbarn/bb-storage/pkg/blobstore/configuration"
	"github.com/buildbarn/bb-storage/pkg/util"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	build "google.golang.org/genproto/googleapis/devtools/build/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type streamState struct {
	committedSequences int64
	bazelBuildEvents   *bytes.Buffer
}

type buildEventServer struct {
	instanceName              string
	contentAddressableStorage blobstore.BlobAccess
	actionCache               ac.ActionCache

	lock    sync.Mutex
	streams map[string]*streamState
}

func (bes *buildEventServer) PublishLifecycleEvent(ctx context.Context, in *build.PublishLifecycleEventRequest) (*empty.Empty, error) {
	// For now, completely ignore lifecycle events.
	return &empty.Empty{}, nil
}

func (bes *buildEventServer) processBuildToolEvent(ctx context.Context, in *build.PublishBuildToolEventStreamRequest) (*build.PublishBuildToolEventStreamResponse, error) {
	bes.lock.Lock()
	defer bes.lock.Unlock()

	streamID := in.OrderedBuildEvent.StreamId
	key := proto.MarshalTextString(streamID)
	state, ok := bes.streams[key]
	if ok {
		if in.OrderedBuildEvent.SequenceNumber < state.committedSequences+1 {
			// Historical event that was retransmitted. Nothing to do.
			return &build.PublishBuildToolEventStreamResponse{
				StreamId:       in.OrderedBuildEvent.StreamId,
				SequenceNumber: state.committedSequences,
			}, nil
		} else if in.OrderedBuildEvent.SequenceNumber > state.committedSequences+1 {
			// Event from the future.
			return nil, status.Error(codes.InvalidArgument, "Event has sequence number from the future")
		}
	} else {
		if in.OrderedBuildEvent.SequenceNumber != 1 {
			return nil, status.Error(codes.DataLoss, "Stream is not known by the server")
		}
		state = &streamState{
			committedSequences: 0,
			bazelBuildEvents:   bytes.NewBuffer(nil),
		}
		bes.streams[key] = state
	}

	switch buildEvent := in.OrderedBuildEvent.Event.Event.(type) {
	case *build.BuildEvent_ComponentStreamFinished:
		log.Print("BuildTool: ComponentStreamFinished: ", buildEvent.ComponentStreamFinished)

		// Convert the invocation ID to a digest, so that we can
		// create fictive AC entries for this stream.
		hash := sha256.Sum256([]byte(streamID.InvocationId))
		actionDigest, err := util.NewDigest(bes.instanceName, &remoteexecution.Digest{
			Hash: hex.EncodeToString(hash[:]),
		})
		if err != nil {
			return nil, err
		}

		// Write the full stream into the CAS.
		data := state.bazelBuildEvents.Bytes()
		digestGenerator := actionDigest.NewDigestGenerator()
		digestGenerator.Write(data)
		streamDigest := digestGenerator.Sum()
		if err := bes.contentAddressableStorage.Put(
			ctx, streamDigest, streamDigest.GetSizeBytes(),
			ioutil.NopCloser(bytes.NewBuffer(data))); err != nil {
			return nil, err
		}

		// Write a fictive entry in the AC, where the full
		// stream is attached as an output file.
		if err := bes.actionCache.PutActionResult(
			ctx,
			actionDigest,
			&remoteexecution.ActionResult{
				OutputFiles: []*remoteexecution.OutputFile{
					{
						Path:   "build-event-stream",
						Digest: streamDigest.GetPartialDigest(),
					},
				},
			}); err != nil {
			return nil, err
		}

		delete(bes.streams, key)
	case *build.BuildEvent_BazelEvent:
		var bazelBuildEvent buildeventstream.BuildEvent
		if err := ptypes.UnmarshalAny(buildEvent.BazelEvent, &bazelBuildEvent); err != nil {
			return nil, err
		}
		if _, err := pbutil.WriteDelimited(state.bazelBuildEvents, &bazelBuildEvent); err != nil {
			return nil, err
		}
	default:
		log.Print("Received unknown BuildToolEvent")
	}

	state.committedSequences++
	return &build.PublishBuildToolEventStreamResponse{
		StreamId:       in.OrderedBuildEvent.StreamId,
		SequenceNumber: state.committedSequences,
	}, nil
}

func (bes *buildEventServer) PublishBuildToolEventStream(stream build.PublishBuildEvent_PublishBuildToolEventStreamServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			log.Print(err)
			return err
		}
		response, err := bes.processBuildToolEvent(stream.Context(), in)
		if err != nil {
			log.Print(err)
			return err
		}
		if err := stream.Send(response); err != nil {
			log.Print(err)
			return err
		}
	}
}

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
		instanceName:              "bb-event-logger",
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

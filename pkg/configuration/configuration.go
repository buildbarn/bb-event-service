package configuration

import (
	"os"

	pb "github.com/buildbarn/bb-event-service/pkg/proto/configuration/bb_event_service"
	"github.com/golang/protobuf/jsonpb"
)

// GetEventServiceConfiguration reads the configuration from file and fill in default values.
func GetEventServiceConfiguration(path string) (*pb.EventServiceConfiguration, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var eventServiceConfiguration pb.EventServiceConfiguration
	if err := jsonpb.Unmarshal(file, &eventServiceConfiguration); err != nil {
		return nil, err
	}
	setDefaultEventServiceValues(&eventServiceConfiguration)
	return &eventServiceConfiguration, err
}

func setDefaultEventServiceValues(eventServiceConfiguration *pb.EventServiceConfiguration) {
	if eventServiceConfiguration.MetricsListenAddress == "" {
		eventServiceConfiguration.MetricsListenAddress = ":80"
	}
	if eventServiceConfiguration.GrpcListenAddress == "" {
		eventServiceConfiguration.GrpcListenAddress = ":8983"
	}
}

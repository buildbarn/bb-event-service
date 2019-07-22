package configuration

import (
	pb "github.com/buildbarn/bb-event-service/pkg/proto/configuration/bb_event_service"
	"github.com/buildbarn/bb-storage/pkg/util"
)

// GetEventServiceConfiguration reads the jsonnet configuration from file,
// renders it, and fills in default values.
func GetEventServiceConfiguration(path string) (*pb.EventServiceConfiguration, error) {
	var eventServiceConfiguration pb.EventServiceConfiguration
	if err := util.UnmarshalConfigurationFromFile(path, &eventServiceConfiguration); err != nil {
		return nil, util.StatusWrap(err, "Failed to retrieve configuration")
	}
	setDefaultEventServiceValues(&eventServiceConfiguration)
	return &eventServiceConfiguration, nil
}

func setDefaultEventServiceValues(eventServiceConfiguration *pb.EventServiceConfiguration) {
	if eventServiceConfiguration.MetricsListenAddress == "" {
		eventServiceConfiguration.MetricsListenAddress = ":80"
	}
	if eventServiceConfiguration.GrpcListenAddress == "" {
		eventServiceConfiguration.GrpcListenAddress = ":8983"
	}
}

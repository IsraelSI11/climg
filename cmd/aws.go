package cmd

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// loadAWSConfig resolves credentials the same way the AWS CLI does --
// environment variables, then the shared config/credentials files, then
// container/instance metadata -- pinned to the given region.
func loadAWSConfig(ctx context.Context, region string) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx, config.WithRegion(region))
}

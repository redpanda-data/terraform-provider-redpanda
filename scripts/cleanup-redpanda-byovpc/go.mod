module github.com/redpanda-data/terraform-provider-redpanda/scripts/cleanup-redpanda-byovpc

go 1.23

require (
	github.com/aws/aws-sdk-go-v2 v1.36.3
	github.com/aws/aws-sdk-go-v2/config v1.28.6
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.52.0
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.37.1
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.191.0
	github.com/aws/aws-sdk-go-v2/service/eks v1.54.0
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.43.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.38.2
	github.com/aws/aws-sdk-go-v2/service/s3 v1.68.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.2
	github.com/fatih/color v1.18.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.7 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.47 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.4.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.10.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.6 // indirect
	github.com/aws/smithy-go v1.22.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.25.0 // indirect
)

module github.com/redpanda-data/terraform-provider-redpanda

go 1.22.4

require (
	buf.build/gen/go/redpandadata/cloud/grpc/go v1.5.1-20240820164511-1d38ab631312.1
	buf.build/gen/go/redpandadata/cloud/protocolbuffers/go v1.34.2-20240820164511-1d38ab631312.2
	buf.build/gen/go/redpandadata/dataplane/grpc/go v1.5.1-20240823133854-b83c57715214.1
	buf.build/gen/go/redpandadata/dataplane/protocolbuffers/go v1.34.2-20240823133854-b83c57715214.2
	github.com/davecgh/go-spew v1.1.1
	github.com/golang/mock v1.6.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
	github.com/hashicorp/terraform-plugin-docs v0.18.0
	github.com/hashicorp/terraform-plugin-framework v1.10.0
	github.com/hashicorp/terraform-plugin-framework-validators v0.13.0
	github.com/hashicorp/terraform-plugin-go v0.23.0
	github.com/hashicorp/terraform-plugin-log v0.9.0
	github.com/hashicorp/terraform-plugin-testing v1.10.0
	github.com/redpanda-data/redpanda/src/go/rpk v0.0.0-20240715191109-e3ca3047d5b7
	github.com/stretchr/testify v1.9.0
	golang.org/x/time v0.5.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240711142825-46eb208f015d
	google.golang.org/grpc v1.65.0
	google.golang.org/protobuf v1.34.2
)

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.34.2-20240508200655-46a4cf4ba109.2 // indirect
	buf.build/gen/go/grpc-ecosystem/grpc-gateway/protocolbuffers/go v1.34.2-20240617172850-a48fcebcf8f1.2 // indirect
	buf.build/gen/go/redpandadata/common/protocolbuffers/go v1.34.2-20240715174743-9c0afe867874.2 // indirect
	github.com/Kunde21/markdownfmt/v3 v3.1.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.0 // indirect
	github.com/Masterminds/sprig/v3 v3.2.3 // indirect
	github.com/ProtonMail/go-crypto v1.1.0-alpha.2 // indirect
	github.com/agext/levenshtein v1.2.2 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/bgentry/speakeasy v0.1.0 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/fatih/color v1.17.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/cli v1.1.6 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-checkpoint v0.5.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-cty v1.4.1-0.20200414143053-d3edf31b6320 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-plugin v1.6.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/hc-install v0.8.0 // indirect
	github.com/hashicorp/hcl/v2 v2.21.0 // indirect
	github.com/hashicorp/logutils v1.0.0 // indirect
	github.com/hashicorp/terraform-exec v0.21.0 // indirect
	github.com/hashicorp/terraform-json v0.22.1 // indirect
	github.com/hashicorp/terraform-plugin-sdk/v2 v2.34.0 // indirect
	github.com/hashicorp/terraform-registry-address v0.2.3 // indirect
	github.com/hashicorp/terraform-svchost v0.1.1 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/huandu/xstrings v1.3.3 // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/oklog/run v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/posener/complete v1.2.3 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/russross/blackfriday v1.6.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/vmihailenco/msgpack v4.0.4+incompatible // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/yuin/goldmark v1.6.0 // indirect
	github.com/yuin/goldmark-meta v1.1.0 // indirect
	github.com/zclconf/go-cty v1.15.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/mod v0.19.0 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.23.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto v0.0.0-20240711142825-46eb208f015d // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240711142825-46eb208f015d // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

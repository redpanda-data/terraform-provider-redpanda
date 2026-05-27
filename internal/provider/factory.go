// Package provider exposes the muxed Terraform provider server used by both
// the production binary (main.go) and the testing framework factory map.
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-mux/tf6muxserver"
	"github.com/redpanda-data/terraform-provider-redpanda/redpanda"
)

// Option configures the muxed-server factory. Production callers pass none;
// the integration tier supplies WithProviderOption(redpanda.WithDialer(...))
// + WithProviderOption(redpanda.WithSkipAuth()) to drive in-memory tests.
type Option func(*config)

type config struct {
	providerOpts []redpanda.Option
}

// WithProviderOption forwards a redpanda.Option through to redpanda.New.
func WithProviderOption(o redpanda.Option) Option {
	return func(c *config) { c.providerOpts = append(c.providerOpts, o) }
}

// NewMuxedServer returns a factory that wraps the framework provider in
// tf6muxserver for the production serve path and tests.
func NewMuxedServer(ctx context.Context, cloudEnv, version string, opts ...Option) func() (tfprotov6.ProviderServer, error) {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}
	return func() (tfprotov6.ProviderServer, error) {
		return tf6muxserver.NewMuxServer(ctx,
			providerserver.NewProtocol6(redpanda.New(ctx, cloudEnv, version, cfg.providerOpts...)()),
		)
	}
}

// ProtoV6ProviderFactories returns the framework-test factory map keyed by
// provider address.
func ProtoV6ProviderFactories(ctx context.Context, cloudEnv, version string, opts ...Option) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"redpanda": NewMuxedServer(ctx, cloudEnv, version, opts...),
	}
}

package namespace

import (
	ujconfig "github.com/crossplane/upjet/v2/pkg/config"
)

// Configure configures temporalcloud_namespace for the namespaced provider.
func Configure(p *ujconfig.Provider) {
	p.AddResourceConfigurator("temporalcloud_namespace", func(r *ujconfig.Resource) {
		r.ShortGroup = "namespace"
		r.Kind = "Namespace"
	})
}

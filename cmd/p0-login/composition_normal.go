//go:build !e2e

package main

import (
	"my-bilibili/internal/account"
	accountapi "my-bilibili/internal/adapters/bilibili/account"
	"my-bilibili/internal/adapters/credentials"
)

const syntheticBuild = false

func compose(o options) (backend, error) {
	if o.synthetic {
		return backend{}, account.Invalid
	}
	gateway := accountapi.New(nil)
	return backend{gateway: gateway, store: credentials.New(), close: gateway.Close}, nil
}

package memcache_fx

import (
	"go.uber.org/fx"
	mem "vivu/pkg/memcache"
)

var Module = fx.Provide(provideMemcacheClient)

func provideMemcacheClient() mem.ResetTokenStore {
	return mem.NewResetTokens()
}

package types

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/ibc-go/v8/modules/core/exported"
)

// The router is a map from module name to the LightClientModule
// which contains all the module-defined callbacks required by ICS-26
type Router struct {
	routes        map[string]exported.LightClientModule
	storeProvider exported.ClientStoreProvider
}

func NewRouter(key storetypes.StoreKey) *Router {
	return &Router{
		routes:        make(map[string]exported.LightClientModule),
		storeProvider: NewStoreProvider(key),
	}
}

// AddRoute adds LightClientModule for a given module name. It returns the Router
// so AddRoute calls can be linked. It will panic if the Router is sealed.
func (rtr *Router) AddRoute(clientType string, module exported.LightClientModule) *Router {
	if rtr.HasRoute(clientType) {
		panic(fmt.Errorf("route %s has already been registered", module))
	}

	rtr.routes[clientType] = module

	module.RegisterStoreProvider(rtr.storeProvider)
	return rtr
}

// HasRoute returns true if the Router has a module registered or false otherwise.
func (rtr *Router) HasRoute(module string) bool {
	_, ok := rtr.routes[module]
	return ok
}

// GetRoute returns the LightClientModule registered for the client type
// associated with the clientID.
func (rtr *Router) GetRoute(clientID string) (exported.LightClientModule, bool) {
	clientType, _, err := ParseClientIdentifier(clientID)
	if err != nil {
		return nil, false
	}

	if !rtr.HasRoute(clientType) {
		return nil, false
	}
	return rtr.routes[clientType], true
}

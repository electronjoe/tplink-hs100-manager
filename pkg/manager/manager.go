// package manager offers persistent, stable management of a collection of TP-Link HS100 or HS110 smart plugs.
// The Manager is expected to manage the state (desired vs actual) and connectivity with a set of requested manageLabels (established by "name" field in TP-Link config).
package manager 

import (
	"context"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/jaedle/golang-tplink-hs100/pkg/hs100"
)

// Manager is responsible for maintaining cached state of network connected smart plugs, periodically polling to re-validate both plug presence and state (e.g. on/off).
//
// Create a new Manager with smartplug.New(...).
// The Manager manages exclusively the TP-Link smartplugs with names found in the manageLabels set, passed to New.
// The Manager discovers the TP-Link devices on the busnets provided to New.
// Note that because the smartplug IPs may change, we rely on the label to be unique and stable.  Perhaps a mode can be offered that pins IP addresses (would require DHCP server config by installer). Use of labels does present a risk of collision, though so does IP addressses if DHCP is misconfigured.
//
// The Manager does not operate without the user calling the blocking function Run(...). Presumably from an external goroutine.
//
// The Manager is coroutine-safe.
type Manager struct {
	/////////
	// The following are immutable at-New parameters
	/////////

	// discovery is a provided device discovery function provided to New.
	discovery func() ([]*hs100.Hs100, error)
	// labels contains the human-readable set of labels to be managed.  For E.g. TP-Link hs100 smartplugs, this is their configured name.
	manageLabels map[string]struct{}
	// pollingInterval indicates how regularly we poll all smart plugs to check online status and confirm state.
	pollingInterval time.Duration

	/////////
	// The following are mutable state protected by stateMutex.
	/////////

	// stateMutex protects access to the following mutable state.
	stateMutex sync.Mutex
	// desiredStates indicates what the desired states are, which may not be current states if e.g. a smartplug fell offline. The map is indexed by smartplug label.
	desiredStates map[string]bool
	// lastStates indicates the most recent fetched state for each smartplug. This is polled at pollingInterval.
	lastStates map[string]bool
	// smartplugs presents a mapping of labels to smartplug control objects, indexed by label.
	smartplugs map[string]*hs100.Hs100
	// disconnected presents a set of smartplug labels which need reconnect.
	disconnected map[string]struct{}
}

// New generates a new HS100 smart plug manager.
func (*Manager) New(discovery func() ([]*hs100.Hs100, error), manageLabels map[string]struct{}, pollingInterval time.Duration) *Manager {
	smartplugs := make(map[string]*hs100.Hs100, len(manageLabels))
	desiredStates := make(map[string]bool, len(manageLabels))
	lastStates := make(map[string]bool, len(manageLabels))
	disconnected := make(map[string]struct{})

	return &Manager{
		discovery:       discovery,
		manageLabels:    manageLabels,
		desiredStates:   desiredStates,
		lastStates:      lastStates,
		pollingInterval: pollingInterval,
		smartplugs:      smartplugs,
		disconnected:    disconnected,
	}
}

// attemptReconnect scans through discoverable plugs and moves any discovered m.disconnected plugs to m.smartplugs.
func (m *Manager) attemptReconnect() {
	if len(m.disconnected) == 0 {
		return
	}

	discoveredPlugs, err := m.discovery()
	if err != nil {
		// TODO: should this be Fatal, or might we recover?
		glog.Fatalf("Failed in hs100.Discover, err: %s\n", err)
	}

	for _, plug := range discoveredPlugs {
		name, err := plug.GetName()
		if err != nil {
			glog.Warningf("Getname on smartplug with address %q failed with err: %s, skipping smartplug", d.Address, err)
			continue
		}

		m.admitSmartplugIfDisconnected(name, plug)
	}
}

// admitSmartplugIfDisconnected moves a plug from the disconnected set to the smartplug set if present in the disconnected set.
func (m *Manager) admitSmartplugIfDisconnected(name string, plug *hs100.Hs100) {
	m.stateMutex.Lock()
	defer m.stateMutex.Unlock()
	if _, ok := m.disconnected[name]; ok {
		// Found a disconnected device! Add to healthy set.
		delete(m.disconnected, name)
		m.smartplugs[name] = plug
	}
}

// Run is a blocking function which is used to poll the state of all managed smart outlets, monitor their presence and re-apply their state if inconsistent.
func (m *Manager) Run(ctx context.Context) {
	ticker := time.NewTicker(m.pollingInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case _ = <-ticker.C:
			if len(m.disconnected) != 0 {
				glog.V(4).Info("Attempting to reconnect to disconnected set of plugs %v.", m.disconnected)
				m.attemptReconnect()
			}
			glog.V(4).Info("Validating health andstate of smart plugs, applying desired state.")
			m.updateState()
		}
	}
}

// updateState cycles through the healthy connected plugs lisetd in smartplugs, updates their cached state (on / off) and aligns them with desired state (updates on / off if necessary). Any failures on access of the smartplug causes the plug to be removed from smartplugs and added by label to the disconnected set (where periodic attemptReconnect() will try to restore health by re-discovery of the plug by label).
func (m *Manager) updateState() {
	// Gather state of the plugs, collect slice of plugs that are inaccessible
	tryReconnectNames := make([]string, 0, len(m.smartplugs))
	for name, plug := range m.smartplugs {
		isOn, err := plug.IsOn()
		if err != nil {
			glog.Warningf("IsOn on smartplug with label %q, address %q failed with err: %s", name, plug.Address, err)
			m.stateMutex.Lock()
			defer m.stateMutex.Unlock()
			tryReconnectNames = append(tryReconnectNames, name)
			delete(m.smartplugs, name)
			continue
		}

		// Protect modification by lock use
		{
			m.stateMutex.Lock()
			defer m.stateMutex.Unlock()
			m.lastStates[name] = isOn
		}
		// desiredStates[name] = false
	}

	// Attempt to reconnect to any failing plugs
}

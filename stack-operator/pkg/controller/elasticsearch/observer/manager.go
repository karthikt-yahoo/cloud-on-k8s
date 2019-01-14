package observer

import (
	"sync"

	"github.com/elastic/stack-operators/stack-operator/pkg/controller/elasticsearch/client"
	"k8s.io/apimachinery/pkg/types"
)

// Manager for a set of observers
type Manager struct {
	observers map[types.NamespacedName]*Observer
	lock      sync.RWMutex
	settings  Settings
}

// NewManager returns a new manager
func NewManager(settings Settings) *Manager {
	return &Manager{
		observers: make(map[types.NamespacedName]*Observer),
		lock:      sync.RWMutex{},
		settings:  settings,
	}
}

// ObservedStateResolver returns the last known state of the given cluster,
// as expected by the main reconciliation driver
func (m *Manager) ObservedStateResolver(clusterName types.NamespacedName, esClient *client.Client) State {
	return m.Observe(clusterName, esClient).LastState()
}

// Observe gets or create a cluster state observer for the given cluster
// In case something has changed in the given esClient (eg. different caCert), the observer is recreated accordingly
func (m *Manager) Observe(clusterName types.NamespacedName, esClient *client.Client) *Observer {
	m.lock.RLock()
	observer, exists := m.observers[clusterName]
	m.lock.RUnlock()

	switch {
	case !exists:
		return m.createObserver(clusterName, esClient)
	case exists && !observer.esClient.Equal(esClient):
		log.Info("Replacing observer HTTP client", "cluster", clusterName)
		m.StopObserving(clusterName)
		return m.createObserver(clusterName, esClient)
	default:
		return observer
	}
}

// createObserver creates a new observer according to the given arguments,
// and create/replace its entry in the observers map
func (m *Manager) createObserver(clusterName types.NamespacedName, esClient *client.Client) *Observer {
	observer := NewObserver(clusterName, esClient, m.settings)
	m.lock.Lock()
	m.observers[clusterName] = observer
	m.lock.Unlock()
	return observer
}

// StopObserving stops and deletes the observer for the given cluster
// aimed to be called automatically by a finalizer
func (m *Manager) StopObserving(clusterName types.NamespacedName) {
	m.lock.RLock()
	observer, exists := m.observers[clusterName]
	m.lock.RUnlock()
	if !exists {
		return
	}
	observer.Stop()
	m.lock.Lock()
	delete(m.observers, clusterName)
	m.lock.Unlock()
}

// List returns the names of clusters currently observed
func (m *Manager) List() []types.NamespacedName {
	m.lock.RLock()
	defer m.lock.RUnlock()
	list := []types.NamespacedName{}
	for name := range m.observers {
		list = append(list, name)
	}
	return list
}

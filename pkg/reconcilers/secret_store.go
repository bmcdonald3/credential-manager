package reconcilers

import (
	"sync"

	"github.com/openchami/fabrica/pkg/events"
	"github.com/openchami/fabrica/pkg/reconcile"
	"github.com/user/credential-manager/pkg/secrets"
)

var (
	defaultSecretStoreMu sync.RWMutex
	defaultSecretStore   secrets.SecretStore = secrets.NewLocalSecretStore()
)

// SetDefaultSecretStore configures the SecretStore used by reconcilers.
func SetDefaultSecretStore(store secrets.SecretStore) {
	if store == nil {
		return
	}
	defaultSecretStoreMu.Lock()
	defer defaultSecretStoreMu.Unlock()
	defaultSecretStore = store
}

func (r *BmcCredentialReconciler) secretStore() secrets.SecretStore {
	defaultSecretStoreMu.RLock()
	defer defaultSecretStoreMu.RUnlock()
	return defaultSecretStore
}

// NewBmcCredentialReconciler creates a BmcCredential reconciler with an injected SecretStore.
func NewBmcCredentialReconciler(client reconcile.ClientInterface, eventBus events.EventBus, store secrets.SecretStore) *BmcCredentialReconciler {
	SetDefaultSecretStore(store)
	return NewDefaultBmcCredentialReconciler(client, eventBus)
}

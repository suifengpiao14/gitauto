package gitauto

import (
	"fmt"
	"sync"

	"github.com/go-git/go-git/v5/plumbing/transport"
)

type authContainer struct {
	authMap sync.Map
}

func (a *authContainer) Register(hostname string, auth transport.AuthMethod) {
	a.authMap.Store(hostname, auth)
}
func (a *authContainer) Get(hostname string) (auth transport.AuthMethod, ok bool) {
	value, ok := a.authMap.Load(hostname)
	if !ok {
		return nil, false
	}
	auth, ok = value.(transport.AuthMethod)
	if !ok {
		return nil, false
	}

	return auth, true
}

var _authContainer = authContainer{}

func RegisterAuth(username string, hostname string, auth transport.AuthMethod) {
	key := getAuthMapKey(username, hostname)
	_authContainer.Register(key, auth)
}

func GetAuth(username string, hostname string) (auth transport.AuthMethod, ok bool) {
	key := getAuthMapKey(username, hostname)
	auth, ok = _authContainer.Get(key)
	return auth, ok
}

func getAuthMapKey(username string, hostname string) (key string) {
	return fmt.Sprintf("%s@%s", username, hostname)
}

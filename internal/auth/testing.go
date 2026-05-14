package auth

import (
	"github.com/pheelee/gslb/internal/store"
	"github.com/pheelee/gslb/internal/types"
)

// OIDCServiceForTest constructs an OIDCService suitable for unit/integration
// tests: it skips provider discovery and only wires JWT signing/parsing and the
// store. The zero-value *oidc.Provider and verifier fields are intentionally
// nil — only CreateSession and ParseSession are safe to call on the result.
func OIDCServiceForTest(oidcCfg types.OIDCConfig, jwtCfg types.JWTConfig, s store.Store) OIDCService {
	return OIDCService{
		oidcCfg:   oidcCfg,
		jwtCfg:    jwtCfg,
		store:     s,
		jwtSecret: []byte(jwtCfg.Secret),
	}
}

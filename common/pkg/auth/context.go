package auth

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
)

// AuthContext holds the authenticated identity and permissions.
// Populated by Middleware, then available to RequirePermission and
// all downstream handlers via GetAuth / GetAuthFromContext.
type AuthContext struct {
	IdentityID   uint
	IdentityType IdentityType
	ClientID     *uint
	EmployeeID   *uint
	Permissions  []permission.Permission
}

type authKeyType struct{}

var authKey = authKeyType{}

// SetAuth stores the authenticated identity in both the Gin context and the
// request's stdlib context. The Gin context is used by middleware (GetAuth),
// and the stdlib context is used by service-layer code (GetAuthFromContext).
func SetAuth(c *gin.Context, auth *AuthContext) {
	c.Set(authKey, auth)
	ctx := context.WithValue(c.Request.Context(), authKey, auth)
	c.Request = c.Request.WithContext(ctx)
}

// SetAuthOnContext stores AuthContext into a stdlib context.
// This is intended for use in service-layer unit tests where
// there is no Gin context available.
func SetAuthOnContext(ctx context.Context, ac *AuthContext) context.Context {
	return context.WithValue(ctx, authKey, ac)
}

// GetAuth retrieves the authenticated identity from the Gin context.
func GetAuth(c *gin.Context) *AuthContext {
	val, exists := c.Get(authKey)
	if !exists {
		return nil
	}

	auth, ok := val.(*AuthContext)
	if !ok {
		return nil
	}

	return auth
}

// GetAuthFromContext retrieves the authenticated identity from a stdlib context.
func GetAuthFromContext(ctx context.Context) *AuthContext {
	val := ctx.Value(authKey)
	if val == nil {
		return nil
	}

	auth, ok := val.(*AuthContext)
	if !ok {
		return nil
	}

	return auth
}

// GetSubjectFromContext resolves the authenticated subject id from stdlib context.
// For client identities this is ClientID, and for employee identities this is EmployeeID.
func GetSubjectFromContext(ctx context.Context) (uint, error) {
	authCtx := GetAuthFromContext(ctx)
	if authCtx == nil {
		return 0, errors.UnauthorizedErr("not authenticated")
	}

	if authCtx.IdentityType == IdentityClient {
		if authCtx.ClientID == nil {
			return 0, errors.UnauthorizedErr("not authenticated")
		}

		return *authCtx.ClientID, nil
	}

	if authCtx.IdentityType == IdentityEmployee {
		if authCtx.EmployeeID == nil {
			return 0, errors.UnauthorizedErr("not authenticated")
		}
		return *authCtx.EmployeeID, nil
	}

	return 0, errors.ForbiddenErr("access denied for this identity type")
}

// GetIdentityIDFromContext resolves the authenticated identity id (Identity.ID)
// from stdlib context. Unlike GetSubjectFromContext this is the single global id
// shared across clients and employees — use it wherever a user id crosses the
// cross-bank wire so it stays unambiguous between banks.
func GetIdentityIDFromContext(ctx context.Context) (uint, error) {
	authCtx := GetAuthFromContext(ctx)
	if authCtx == nil {
		return 0, errors.UnauthorizedErr("not authenticated")
	}
	if authCtx.IdentityID == 0 {
		return 0, errors.UnauthorizedErr("not authenticated")
	}
	return authCtx.IdentityID, nil
}

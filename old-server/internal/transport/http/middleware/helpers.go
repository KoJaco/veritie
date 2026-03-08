package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	domain_auth "schma.ai/internal/domain/auth"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const principalKey contextKey = "principal"

// GetPrincipal extracts principal from request context
func GetPrincipal(ctx context.Context) (domain_auth.Principal, bool) {
	principal, ok := ctx.Value(principalKey).(domain_auth.Principal)
	return principal, ok
}

func PrincipalFromJWT(ctx context.Context, appId string, authService domain_auth.AuthService) (domain_auth.Principal, error) {
    parsedUUID, err := uuid.Parse(appId)
    if err != nil {
        return domain_auth.Principal{}, err
    }
    return authService.LookupPrincipalByAppID(ctx, pgtype.UUID{Bytes: parsedUUID, Valid: true})
}
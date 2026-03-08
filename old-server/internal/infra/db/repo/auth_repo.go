package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"schma.ai/internal/domain/auth"
	db "schma.ai/internal/infra/db/generated"
	"schma.ai/internal/pkg/logger"
)

// TODO: Define port for this repo with compile time check.


type AuthRepo struct {
	q *db.Queries
}

func NewAuthRepo(pool *pgxpool.Pool) *AuthRepo {
	return &AuthRepo{q: db.New(pool)}
}

var _ auth.Validator = (*AuthRepo)(nil)

func (r *AuthRepo) GetPrincipalByAppID(ctx context.Context, appID pgtype.UUID) (auth.Principal, error) {
	row, err := r.q.GetAppByID(ctx, appID)

	if err != nil {
		logger.Errorf("❌ [DB] GetPrincipalByAppID: Failed to find app by API key: %v", err)
		return auth.Principal{}, fmt.Errorf("auth validate: %w", err)
	}
	logger.ServiceDebugf("DB", "GetPrincipalByAppID: Found app: %s (ID: %s, Account: %s)", row.Name, row.ID, row.AccountID)

	var cfg auth.AppConfig
	logger.ServiceDebugf("DB", "GetPrincipalByAppID: Parsing app config: %s", string(row.Config))

	if len(row.Config) == 0 {
		logger.Warnf("⚠️ [DB] GetPrincipalByAppID: App config is empty, using defaults")
		cfg = auth.AppConfig{
			AllowedOrigins: []string{"http://localhost:3000", "https://localhost:3000"},
			EnabledSchemas: []string{},
			PreferredLLM:   "gemini-2.0-flash",
		}
	} else {
		if err := json.Unmarshal(row.Config, &cfg); err != nil {
			logger.Errorf("❌ [DB] GetPrincipalByAppID: Failed to parse app config: %v", err)
			return auth.Principal{}, fmt.Errorf("auth: parse config: %w", err)
		}
	}

	principal := auth.Principal{
		AppID:          row.ID,
		AccountID:      row.AccountID,
		AppName:        row.Name,
		AppDescription: row.Description,
		AppConfig:      cfg,
	}

	logger.ServiceDebugf("DB", "GetPrincipalByAppID: Created principal: %+v", principal)
	return principal, nil
}


func (r *AuthRepo) ValidateToken(ctx context.Context, token string) (auth.Principal, error) {
	logger.ServiceDebugf("DB", "Looking up API key in database: '%s'", token)
	row, err := r.q.GetAppByAPIKey(ctx, token)
	if err != nil {
		logger.Errorf("❌ [DB] Failed to find app by API key: %v", err)
		return auth.Principal{}, fmt.Errorf("auth validate: %w", err)
	}
	logger.ServiceDebugf("DB", "Found app: %s (ID: %s, Account: %s)", row.Name, row.ID, row.AccountID)

	var cfg auth.AppConfig
	logger.ServiceDebugf("DB", "Parsing app config: %s", string(row.Config))

	// Handle NULL or empty config gracefully
	// TODO: adjust seed data so that we fill in this config appropriately. AllowedOrigins is essential
	if len(row.Config) == 0 {
		logger.Warnf("⚠️ [DB] App config is empty, using defaults")
		cfg = auth.AppConfig{
			AllowedOrigins: []string{"http://localhost:3000", "https://localhost:3000"},
			EnabledSchemas: []string{},
			PreferredLLM:   "gemini-2.0-flash",
		}
	} else {
		if err := json.Unmarshal(row.Config, &cfg); err != nil {
			logger.Errorf("❌ [DB] Failed to parse app config: %v", err)
			return auth.Principal{}, fmt.Errorf("auth: parse config: %w", err)
		}
	}

	principal := auth.Principal{
		AppID:          row.ID,
		AccountID:      row.AccountID,
		AppName:        row.Name,
		AppDescription: row.Description,
		AppConfig:      cfg,
	}

	logger.ServiceDebugf("DB", "Created principal: %+v", principal)
	return principal, nil
}

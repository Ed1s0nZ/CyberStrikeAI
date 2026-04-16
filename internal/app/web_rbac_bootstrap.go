package app

import (
	"context"

	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/security"
)

func bootstrapWebRBAC(ctx context.Context, db *database.DB, legacyPassword string) error {
	db.SetWebPermissionNormalizer(security.NormalizeWebPermissions)

	if err := security.EnsureBootstrapAdmin(ctx, db, legacyPassword); err != nil {
		return err
	}

	return security.NormalizePersistedWebRBACPermissions(ctx, db)
}

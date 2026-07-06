package handlers

import (
	"context"
	"database/sql"
	"errors"

	"github.com/sms-relay/server/internal/crypto"
	"github.com/sms-relay/server/internal/db"
)

type passwordLookupResult struct {
	user         db.User
	needsUpgrade bool
}

func (h *Handler) findUserByPassword(ctx context.Context, password string) (passwordLookupResult, error) {
	primaryFP := crypto.PasswordFingerprint(password, h.cfg.PasswordPepper)
	user, err := h.queries.GetUserByFingerprint(ctx, primaryFP)
	if err == nil {
		return passwordLookupResult{user: user}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return passwordLookupResult{}, err
	}

	if h.cfg.PasswordPepper != h.cfg.JWTSecret {
		jwtFP := crypto.PasswordFingerprint(password, h.cfg.JWTSecret)
		user, err = h.queries.GetUserByFingerprint(ctx, jwtFP)
		if err == nil {
			return passwordLookupResult{user: user, needsUpgrade: true}, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return passwordLookupResult{}, err
		}
	}

	legacyFP := crypto.LegacyPasswordFingerprint(password)
	user, err = h.queries.GetUserByFingerprint(ctx, legacyFP)
	if err != nil {
		return passwordLookupResult{}, err
	}
	return passwordLookupResult{user: user, needsUpgrade: true}, nil
}

func (h *Handler) upgradePasswordFingerprint(ctx context.Context, userID, password string) {
	fp := crypto.PasswordFingerprint(password, h.cfg.PasswordPepper)
	_ = h.queries.UpdateUserPasswordFingerprint(ctx, db.UpdateUserPasswordFingerprintParams{
		PasswordFingerprint: fp,
		ID:                  userID,
	})
}

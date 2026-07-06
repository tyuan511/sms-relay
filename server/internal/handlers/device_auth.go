package handlers

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sms-relay/server/internal/applog"
	"github.com/sms-relay/server/internal/auth"
	"github.com/sms-relay/server/internal/db"
)

type deviceAuthRequest struct {
	MasterPassword string `json:"master_password"`
	DeviceName     string `json:"device_name"`
	DeviceClientID string `json:"device_client_id"`
}

type deviceAuthResponse struct {
	DeviceToken string `json:"device_token"`
	DeviceID    string `json:"device_id"`
	TokenType   string `json:"token_type"`
}

func (h *Handler) AuthDevice(c *fiber.Ctx) error {
	var req deviceAuthRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "invalid request"})
	}
	req.MasterPassword = strings.TrimSpace(req.MasterPassword)
	req.DeviceName = strings.TrimSpace(req.DeviceName)
	req.DeviceClientID = strings.TrimSpace(req.DeviceClientID)
	if req.MasterPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "master_password is required"})
	}
	if req.DeviceName == "" {
		req.DeviceName = "Android"
	}

	lookup, err := h.findUserByPassword(c.Context(), req.MasterPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"detail": "invalid credentials"})
		}
		applog.ReqError("handler.auth", "device_auth", c, fiber.StatusInternalServerError, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "database error"})
	}
	if !auth.CheckPassword(lookup.user.PasswordHash, req.MasterPassword) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"detail": "invalid credentials"})
	}
	if lookup.needsUpgrade {
		h.upgradePasswordFingerprint(c.Context(), lookup.user.ID, req.MasterPassword)
	}

	deviceID, err := h.ensureDeviceRecord(c, lookup.user.ID, req.DeviceName, req.DeviceClientID)
	if err != nil {
		applog.ReqError("handler.auth", "device_auth", c, fiber.StatusInternalServerError, err, "step", "register_device")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to register device"})
	}

	token, err := auth.CreateDeviceToken(lookup.user.ID, deviceID, h.cfg.JWTSecret, 365*24*time.Hour)
	if err != nil {
		applog.ReqError("handler.auth", "device_auth", c, fiber.StatusInternalServerError, err, "step", "create_token")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to create device token"})
	}

	return c.JSON(deviceAuthResponse{
		DeviceToken: token,
		DeviceID:    deviceID,
		TokenType:   "device",
	})
}

func (h *Handler) ensureDeviceRecord(c *fiber.Ctx, userID, name, clientID string) (string, error) {
	clientIDParam := sql.NullString{String: clientID, Valid: clientID != ""}
	if clientIDParam.Valid {
		existing, err := h.queries.GetDeviceByUserAndClientID(c.Context(), db.GetDeviceByUserAndClientIDParams{
			UserID:   userID,
			ClientID: clientIDParam,
		})
		if err == nil {
			now := time.Now().UTC()
			_ = h.queries.UpdateDeviceLastSeen(c.Context(), db.UpdateDeviceLastSeenParams{
				LastSeenAt: sql.NullTime{Time: now, Valid: true},
				ID:         existing.ID,
			})
			return existing.ID, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}

	existing, err := h.queries.GetDeviceByUserAndName(c.Context(), db.GetDeviceByUserAndNameParams{
		UserID: userID,
		Name:   name,
	})
	if err == nil {
		now := time.Now().UTC()
		if clientIDParam.Valid && existing.ClientID.Valid && existing.ClientID.String != clientID {
			id := uuid.New().String()
			dev, err := h.queries.CreateDevice(c.Context(), db.CreateDeviceParams{
				ID:       id,
				UserID:   userID,
				Name:     name,
				ClientID: clientIDParam,
			})
			if err != nil {
				return "", err
			}
			return dev.ID, nil
		}
		if clientIDParam.Valid && !existing.ClientID.Valid {
			_ = h.queries.UpdateDeviceClientID(c.Context(), db.UpdateDeviceClientIDParams{
				ClientID: clientIDParam,
				ID:       existing.ID,
				UserID:   userID,
			})
		}
		_ = h.queries.UpdateDeviceLastSeen(c.Context(), db.UpdateDeviceLastSeenParams{
			LastSeenAt: sql.NullTime{Time: now, Valid: true},
			ID:         existing.ID,
		})
		return existing.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id := uuid.New().String()
	dev, err := h.queries.CreateDevice(c.Context(), db.CreateDeviceParams{
		ID:       id,
		UserID:   userID,
		Name:     name,
		ClientID: clientIDParam,
	})
	if err != nil {
		return "", err
	}
	return dev.ID, nil
}

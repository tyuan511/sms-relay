package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/sms-relay/server/internal/applog"
	"github.com/sms-relay/server/internal/middleware"
	"github.com/sms-relay/server/internal/services"
)

func (h *Handler) DeviceHeartbeat(c *fiber.Ctx) error {
	userID := middleware.UserID(c)
	deviceID := middleware.DeviceID(c)
	if err := h.msgs.TouchDevice(c.Context(), userID, deviceID); err != nil {
		if errors.Is(err, services.ErrInvalidDevice) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"detail": "device not found"})
		}
		applog.ReqError("handler.device", "heartbeat", c, fiber.StatusInternalServerError, err, "device_id", deviceID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "heartbeat failed"})
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

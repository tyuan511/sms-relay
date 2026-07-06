package handlers

import (
	"database/sql"
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/sms-relay/server/internal/applog"
	"github.com/sms-relay/server/internal/middleware"
	"github.com/sms-relay/server/internal/telegram"
)

func (h *Handler) DestinationAvatar(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.UserID(c)

	dest, err := h.queries.GetDestinationByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"detail": "not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "database error"})
	}
	if dest.UserID != userID {
		applog.ReqError("handler.destination", "avatar", c, fiber.StatusForbidden, nil, "destination_id", id)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"detail": "forbidden"})
	}
	if dest.Platform != "telegram" {
		return c.SendStatus(fiber.StatusNotFound)
	}

	cfg, err := h.decryptTelegramConfig(dest)
	if err != nil || cfg.BotToken == "" {
		return c.SendStatus(fiber.StatusNotFound)
	}

	data, contentType, err := h.tgClient.GetBotAvatar(c.Context(), cfg.BotToken)
	if err != nil {
		if errors.Is(err, telegram.ErrNoAvatar) {
			return c.SendStatus(fiber.StatusNotFound)
		}
		applog.ReqError("handler.destination", "avatar", c, fiber.StatusBadGateway, err, "destination_id", id)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"detail": "failed to fetch avatar"})
	}

	c.Set("Content-Type", contentType)
	c.Set("Cache-Control", "private, max-age=3600")
	return c.Send(data)
}

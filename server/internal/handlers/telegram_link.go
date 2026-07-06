package handlers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sms-relay/server/internal/applog"
	"github.com/sms-relay/server/internal/db"
	"github.com/sms-relay/server/internal/middleware"
	"github.com/sms-relay/server/internal/services"
	"github.com/sms-relay/server/internal/telegram"
)

type telegramLinkRequest struct {
	Name     string `json:"name"`
	BotToken string `json:"bot_token"`
}

type telegramLinkResponse struct {
	LinkID      string `json:"link_id"`
	BotUsername string `json:"bot_username"`
	StartURL    string `json:"start_url"`
	ExpiresAt   string `json:"expires_at"`
}

type telegramLinkStatusResponse struct {
	Status        telegram.LinkStatus `json:"status"`
	BotUsername   string              `json:"bot_username,omitempty"`
	StartURL      string              `json:"start_url,omitempty"`
	Destination   *destinationResponse `json:"destination,omitempty"`
	Error         string              `json:"error,omitempty"`
}

func (h *Handler) StartTelegramLink(c *fiber.Ctx) error {
	var req telegramLinkRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "invalid request"})
	}
	req.Name = strings.TrimSpace(req.Name)
	req.BotToken = strings.TrimSpace(req.BotToken)
	if req.Name == "" || req.BotToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "name and bot_token required"})
	}

	userID := middleware.UserID(c)
	session, err := h.linker.Start(c.Context(), userID, req.Name, req.BotToken)
	if err != nil {
		applog.ReqError("handler.telegram", "start_link", c, fiber.StatusBadRequest, err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(telegramLinkResponse{
		LinkID:      session.ID,
		BotUsername: session.BotUsername,
		StartURL:    session.StartURL,
		ExpiresAt:   session.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

func (h *Handler) GetTelegramLinkStatus(c *fiber.Ctx) error {
	linkID := c.Params("id")
	userID := middleware.UserID(c)

	session, err := h.linker.Get(linkID, userID)
	if err != nil {
		if errors.Is(err, telegram.ErrLinkNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"detail": "link session not found"})
		}
		if errors.Is(err, telegram.ErrLinkForbidden) {
			applog.ReqError("handler.telegram", "link_status", c, fiber.StatusForbidden, err, "link_id", linkID)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"detail": "forbidden"})
		}
		applog.ReqError("handler.telegram", "link_status", c, fiber.StatusInternalServerError, err, "link_id", linkID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to get link status"})
	}

	if session.Status == telegram.LinkStatusPending {
		if pollErr := h.linker.Poll(c.Context(), linkID); pollErr != nil {
			applog.Warn("handler.telegram", "link poll failed",
				"link_id", linkID,
				"user_id", userID,
				"error", pollErr.Error(),
			)
			resp := telegramLinkStatusResponse{
				Status:      session.Status,
				BotUsername: session.BotUsername,
				StartURL:    session.StartURL,
				Error:       pollErr.Error(),
			}
			return c.JSON(resp)
		}
		session, err = h.linker.Get(linkID, userID)
		if err != nil {
			applog.ReqError("handler.telegram", "link_status", c, fiber.StatusInternalServerError, err, "link_id", linkID, "step", "refresh_session")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to get link status"})
		}
	}

	resp := telegramLinkStatusResponse{
		Status:      session.Status,
		BotUsername: session.BotUsername,
		StartURL:    session.StartURL,
		Error:       session.Error,
	}

	if session.Status == telegram.LinkStatusLinked && session.DestinationID != "" {
		dest, err := h.queries.GetDestinationByID(c.Context(), session.DestinationID)
		if err == nil {
			d := h.enrichDestResponse(c.Context(), dest)
			resp.Destination = &d
		}
		return c.JSON(resp)
	}

	if _, chatID, ok := h.linker.BeginComplete(linkID, userID); ok {
		dest, err := h.createTelegramDestination(c, userID, session.Name, session.BotToken, chatID, session.BotUsername)
		if err != nil {
			applog.ReqError("handler.telegram", "complete_link", c, fiber.StatusInternalServerError, err, "link_id", linkID)
			h.linker.Fail(linkID, userID, err.Error())
			return c.JSON(telegramLinkStatusResponse{
				Status: telegram.LinkStatusFailed,
				Error:  err.Error(),
			})
		}
		h.linker.Complete(linkID, userID, dest.ID)
		d := h.enrichDestResponse(c.Context(), dest)
		resp.Status = telegram.LinkStatusLinked
		resp.Destination = &d
		go func() { h.linker.Remove(linkID) }()
		return c.JSON(resp)
	}

	return c.JSON(resp)
}

func (h *Handler) createTelegramDestination(c *fiber.Ctx, userID, name, botToken, chatID, botUsername string) (db.Destination, error) {
	id := uuid.New().String()
	cfg := services.TelegramConfig{
		BotToken:    botToken,
		ChatID:      chatID,
		BotUsername: botUsername,
	}
	nonce, enc, err := services.EncryptDestinationConfig(h.crypto, id, cfg)
	if err != nil {
		return db.Destination{}, err
	}
	row, err := h.queries.CreateDestination(c.Context(), db.CreateDestinationParams{
		ID:          id,
		UserID:      userID,
		Name:        name,
		Platform:    "telegram",
		ConfigNonce: nonce,
		ConfigEnc:   enc,
		KeyVersion:  1,
		Enabled:     1,
	})
	if err != nil {
		return db.Destination{}, err
	}
	return row, nil
}

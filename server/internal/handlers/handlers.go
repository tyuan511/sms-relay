package handlers

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sms-relay/server/internal/applog"
	"github.com/sms-relay/server/internal/auth"
	"github.com/sms-relay/server/internal/crypto"
	"github.com/sms-relay/server/internal/db"
	"github.com/sms-relay/server/internal/middleware"
	"github.com/sms-relay/server/internal/services"
	"github.com/sms-relay/server/internal/sse"
	"github.com/sms-relay/server/internal/telegram"
	"github.com/valyala/fasthttp"
)

type Handler struct {
	cfg      Config
	queries  *db.Queries
	sqlDB    *sql.DB
	msgs     *services.MessageService
	hub      *sse.Hub
	crypto   *crypto.FieldEncryptor
	linker   *telegram.Linker
	tgClient *telegram.Client
}

type Config struct {
	JWTSecret      string
	PasswordPepper string
}

func New(cfg Config, queries *db.Queries, sqlDB *sql.DB, msgs *services.MessageService, hub *sse.Hub, enc *crypto.FieldEncryptor, linker *telegram.Linker, tgClient *telegram.Client) *Handler {
	return &Handler{cfg: cfg, queries: queries, sqlDB: sqlDB, msgs: msgs, hub: hub, crypto: enc, linker: linker, tgClient: tgClient}
}

type tokenResponse struct {
	AccessToken    string `json:"access_token"`
	TokenType      string `json:"token_type"`
	MasterPassword string `json:"master_password,omitempty"`
}

type loginRequest struct {
	MasterPassword string `json:"master_password"`
}

func (h *Handler) Register(c *fiber.Ctx) error {
	const maxAttempts = 10
	for i := 0; i < maxAttempts; i++ {
		password, err := auth.GenerateMasterPassword()
		if err != nil {
			applog.ReqError("handler.auth", "register", c, fiber.StatusInternalServerError, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to generate password"})
		}
		fp := crypto.PasswordFingerprint(password, h.cfg.PasswordPepper)
		if _, err := h.queries.GetUserByFingerprint(c.Context(), fp); err == nil {
			continue
		} else if !errors.Is(err, sql.ErrNoRows) {
			applog.ReqError("handler.auth", "register", c, fiber.StatusInternalServerError, err, "step", "lookup_user")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "database error"})
		}

		hash, err := auth.HashPassword(password)
		if err != nil {
			applog.ReqError("handler.auth", "register", c, fiber.StatusInternalServerError, err, "step", "hash_password")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to hash password"})
		}
		userID := uuid.New().String()
		_, err = h.queries.CreateUser(c.Context(), db.CreateUserParams{
			ID:                  userID,
			PasswordHash:        hash,
			PasswordFingerprint: fp,
		})
		if err != nil {
			continue
		}
		token, err := auth.CreateToken(userID, h.cfg.JWTSecret, 7*24*time.Hour)
		if err != nil {
			applog.ReqError("handler.auth", "register", c, fiber.StatusInternalServerError, err, "step", "create_token")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to create token"})
		}
		return c.Status(fiber.StatusCreated).JSON(tokenResponse{
			AccessToken:    token,
			TokenType:      "bearer",
			MasterPassword: password,
		})
	}
	applog.ReqError("handler.auth", "register", c, fiber.StatusInternalServerError, nil, "step", "create_unique_account")
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to create unique account"})
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil || req.MasterPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "master_password is required"})
	}
	lookup, err := h.findUserByPassword(c.Context(), req.MasterPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"detail": "invalid credentials"})
		}
		applog.ReqError("handler.auth", "login", c, fiber.StatusInternalServerError, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "database error"})
	}
	if !auth.CheckPassword(lookup.user.PasswordHash, req.MasterPassword) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"detail": "invalid credentials"})
	}
	if lookup.needsUpgrade {
		h.upgradePasswordFingerprint(c.Context(), lookup.user.ID, req.MasterPassword)
	}
	token, err := auth.CreateToken(lookup.user.ID, h.cfg.JWTSecret, 7*24*time.Hour)
	if err != nil {
		applog.ReqError("handler.auth", "login", c, fiber.StatusInternalServerError, err, "step", "create_token")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to create token"})
	}
	return c.JSON(tokenResponse{AccessToken: token, TokenType: "bearer"})
}

type inboundRequest struct {
	Sender          string `json:"sender"`
	Body            string `json:"body"`
	ReceivedAt      string `json:"received_at"`
	DeviceName      string `json:"device_name"`
	ClientMessageID string `json:"client_message_id"`
}

func (h *Handler) InboundMessage(c *fiber.Ctx) error {
	var req inboundRequest
	if err := c.BodyParser(&req); err != nil || req.Sender == "" || req.Body == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "sender and body are required"})
	}
	var receivedAt time.Time
	if req.ReceivedAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ReceivedAt)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "invalid received_at"})
		}
		receivedAt = parsed
	}
	userID := middleware.UserID(c)
	view, err := h.msgs.ProcessInbound(c.Context(), userID, services.InboundInput{
		Sender:          req.Sender,
		Body:            req.Body,
		ReceivedAt:      receivedAt,
		DeviceName:      req.DeviceName,
		DeviceID:        middleware.DeviceID(c),
		ClientMessageID: strings.TrimSpace(req.ClientMessageID),
	})
	if err != nil {
		if errors.Is(err, services.ErrInvalidDevice) {
			applog.ReqError("handler.message", "inbound", c, fiber.StatusForbidden, err)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"detail": "invalid device"})
		}
		applog.ReqError("handler.message", "inbound", c, fiber.StatusInternalServerError, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to process message"})
	}
	return c.Status(fiber.StatusCreated).JSON(view)
}

func (h *Handler) ListMessages(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	msgs, err := h.msgs.ListMessages(c.Context(), middleware.UserID(c), int64(limit), int64(offset))
	if err != nil {
		applog.ReqError("handler.message", "list", c, fiber.StatusInternalServerError, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to list messages"})
	}
	if msgs == nil {
		msgs = []services.MessageView{}
	}
	return c.JSON(msgs)
}

func (h *Handler) StreamEvents(c *fiber.Ctx) error {
	userID := middleware.UserID(c)
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	ch := h.hub.Subscribe(userID)
	defer h.hub.Unsubscribe(userID, ch)

	c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
		fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
		_ = w.Flush()

		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case data, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
				if err := w.Flush(); err != nil {
					return
				}
			case <-ticker.C:
				fmt.Fprintf(w, ": ping\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	}))
	return nil
}

type destinationRequest struct {
	Name     string                  `json:"name"`
	Platform string                  `json:"platform"`
	Config   services.TelegramConfig `json:"config"`
	Enabled  *bool                   `json:"enabled"`
}

type destinationResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Platform    string    `json:"platform"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	BotUsername string    `json:"bot_username,omitempty"`
	ChatID      string    `json:"chat_id,omitempty"`
}

func (h *Handler) enrichDestResponse(_ context.Context, d db.Destination) destinationResponse {
	resp := destinationResponse{
		ID:        d.ID,
		Name:      d.Name,
		Platform:  d.Platform,
		Enabled:   d.Enabled == 1,
		CreatedAt: d.CreatedAt,
	}
	if d.Platform != "telegram" {
		return resp
	}
	cfg, err := h.decryptTelegramConfig(d)
	if err != nil {
		return resp
	}
	resp.ChatID = cfg.ChatID
	resp.BotUsername = cfg.BotUsername
	return resp
}

func (h *Handler) decryptTelegramConfig(d db.Destination) (services.TelegramConfig, error) {
	plain, err := h.crypto.Decrypt(d.ConfigNonce, d.ConfigEnc, fmt.Sprintf("config:%s", d.ID))
	if err != nil {
		return services.TelegramConfig{}, err
	}
	var cfg services.TelegramConfig
	if err := json.Unmarshal([]byte(plain), &cfg); err != nil {
		return services.TelegramConfig{}, err
	}
	return cfg, nil
}

func (h *Handler) CreateDestination(c *fiber.Ctx) error {
	var req destinationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "invalid request"})
	}
	if req.Name == "" || req.Platform != "telegram" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "name and telegram platform required"})
	}
	if req.Config.BotToken == "" || req.Config.ChatID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "bot_token and chat_id required"})
	}
	if req.Config.BotUsername == "" && h.tgClient != nil {
		if bot, err := h.tgClient.GetMe(c.Context(), req.Config.BotToken); err == nil {
			req.Config.BotUsername = bot.Username
		}
	}
	userID := middleware.UserID(c)
	id := uuid.New().String()
	nonce, enc, err := services.EncryptDestinationConfig(h.crypto, id, req.Config)
	if err != nil {
		applog.ReqError("handler.destination", "create", c, fiber.StatusInternalServerError, err, "step", "encrypt_config")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "encryption failed"})
	}
	enabled := int64(1)
	row, err := h.queries.CreateDestination(c.Context(), db.CreateDestinationParams{
		ID:          id,
		UserID:      userID,
		Name:        req.Name,
		Platform:    "telegram",
		ConfigNonce: nonce,
		ConfigEnc:   enc,
		KeyVersion:  1,
		Enabled:     enabled,
	})
	if err != nil {
		applog.ReqError("handler.destination", "create", c, fiber.StatusInternalServerError, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to create destination"})
	}
	return c.Status(fiber.StatusCreated).JSON(h.enrichDestResponse(c.Context(), row))
}

func (h *Handler) ListDestinations(c *fiber.Ctx) error {
	rows, err := h.queries.ListDestinationsByUser(c.Context(), middleware.UserID(c))
	if err != nil {
		applog.ReqError("handler.destination", "list", c, fiber.StatusInternalServerError, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to list destinations"})
	}
	out := make([]destinationResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, h.enrichDestResponse(c.Context(), r))
	}
	return c.JSON(out)
}

func (h *Handler) UpdateDestination(c *fiber.Ctx) error {
	id := c.Params("id")
	userID := middleware.UserID(c)
	dest, err := h.queries.GetDestinationByID(c.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"detail": "not found"})
		}
		applog.ReqError("handler.destination", "update", c, fiber.StatusInternalServerError, err, "destination_id", id)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "database error"})
	}
	if dest.UserID != userID {
		applog.ReqError("handler.destination", "update", c, fiber.StatusForbidden, nil, "destination_id", id)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"detail": "forbidden"})
	}

	var req destinationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "invalid request"})
	}

	name := dest.Name
	if req.Name != "" {
		name = req.Name
	}
	enabled := dest.Enabled
	if req.Enabled != nil {
		if *req.Enabled {
			enabled = 1
		} else {
			enabled = 0
		}
	}

	cfg := services.TelegramConfig{BotToken: "", ChatID: ""}
	if len(req.Config.BotToken) > 0 || len(req.Config.ChatID) > 0 {
		if req.Config.BotToken == "" || req.Config.ChatID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"detail": "both bot_token and chat_id required when updating config"})
		}
		cfg = req.Config
	} else {
		plain, err := h.crypto.Decrypt(dest.ConfigNonce, dest.ConfigEnc, fmt.Sprintf("config:%s", dest.ID))
		if err != nil {
			applog.ReqError("handler.destination", "update", c, fiber.StatusInternalServerError, err, "step", "decrypt_config")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "decryption failed"})
		}
		if err := json.Unmarshal([]byte(plain), &cfg); err != nil {
			applog.ReqError("handler.destination", "update", c, fiber.StatusInternalServerError, err, "step", "parse_config")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "invalid stored config"})
		}
	}

	nonce, enc, err := services.EncryptDestinationConfig(h.crypto, dest.ID, cfg)
	if err != nil {
		applog.ReqError("handler.destination", "update", c, fiber.StatusInternalServerError, err, "step", "encrypt_config")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "encryption failed"})
	}
	if err := h.queries.UpdateDestination(c.Context(), db.UpdateDestinationParams{
		Name:        name,
		Enabled:     enabled,
		ConfigNonce: nonce,
		ConfigEnc:   enc,
		KeyVersion:  dest.KeyVersion,
		ID:          id,
		UserID:      userID,
	}); err != nil {
		applog.ReqError("handler.destination", "update", c, fiber.StatusInternalServerError, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "update failed"})
	}
	updated, _ := h.queries.GetDestinationByID(c.Context(), id)
	return c.JSON(h.enrichDestResponse(c.Context(), updated))
}

func (h *Handler) DeleteDestination(c *fiber.Ctx) error {
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
		applog.ReqError("handler.destination", "delete", c, fiber.StatusForbidden, nil, "destination_id", id)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"detail": "forbidden"})
	}
	if err := h.queries.DeleteDestination(c.Context(), db.DeleteDestinationParams{ID: id, UserID: userID}); err != nil {
		applog.ReqError("handler.destination", "delete", c, fiber.StatusInternalServerError, err, "destination_id", id)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "delete failed"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) ListDevices(c *fiber.Ctx) error {
	rows, err := h.queries.ListDevicesByUser(c.Context(), middleware.UserID(c))
	if err != nil {
		applog.ReqError("handler.device", "list", c, fiber.StatusInternalServerError, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"detail": "failed to list devices"})
	}
	now := time.Now().UTC()
	type deviceResp struct {
		ID         string     `json:"id"`
		Name       string     `json:"name"`
		LastSeenAt *time.Time `json:"last_seen_at"`
		Online     bool       `json:"online"`
		CreatedAt  time.Time  `json:"created_at"`
	}
	out := make([]deviceResp, 0, len(rows))
	for _, d := range rows {
		var lastSeen *time.Time
		if d.LastSeenAt.Valid {
			t := d.LastSeenAt.Time
			lastSeen = &t
		}
		out = append(out, deviceResp{
			ID:         d.ID,
			Name:       d.Name,
			LastSeenAt: lastSeen,
			Online:     services.IsDeviceOnline(lastSeen, now),
			CreatedAt:  d.CreatedAt,
		})
	}
	return c.JSON(out)
}

func (h *Handler) Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sms-relay/server/internal/applog"
	"github.com/sms-relay/server/internal/crypto"
	"github.com/sms-relay/server/internal/db"
	"github.com/sms-relay/server/internal/sse"
)

type TelegramConfig struct {
	BotToken    string `json:"bot_token"`
	ChatID      string `json:"chat_id"`
	BotUsername string `json:"bot_username,omitempty"`
}

type MessageService struct {
	queries   *db.Queries
	encryptor *crypto.FieldEncryptor
	hub       *sse.Hub
	client    *http.Client
}

var ErrInvalidDevice = errors.New("invalid device")

func NewMessageService(queries *db.Queries, encryptor *crypto.FieldEncryptor, hub *sse.Hub) *MessageService {
	return &MessageService{
		queries:   queries,
		encryptor: encryptor,
		hub:       hub,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

type InboundInput struct {
	Sender          string
	Body            string
	ReceivedAt      time.Time
	DeviceName      string
	DeviceID        string
	ClientMessageID string
}

type MessageView struct {
	ID         string    `json:"id"`
	Sender     string    `json:"sender"`
	Body       string    `json:"body"`
	ReceivedAt time.Time `json:"received_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func (s *MessageService) ProcessInbound(ctx context.Context, userID string, input InboundInput) (*MessageView, error) {
	deviceID := input.DeviceID
	var err error
	if deviceID == "" {
		deviceID, err = s.ensureDevice(ctx, userID, input.DeviceName)
		if err != nil {
			applog.Error("service.message", "ensure device failed", "user_id", userID, "error", err)
			return nil, err
		}
	} else if err := s.TouchDevice(ctx, userID, deviceID); err != nil {
		if errors.Is(err, ErrInvalidDevice) {
			applog.Warn("service.message", "invalid device", "user_id", userID, "device_id", deviceID)
		} else {
			applog.Error("service.message", "touch device failed", "user_id", userID, "device_id", deviceID, "error", err)
		}
		return nil, err
	}

	if input.ClientMessageID != "" {
		existing, err := s.queries.GetMessageByClientID(ctx, db.GetMessageByClientIDParams{
			UserID:          userID,
			DeviceID:        sql.NullString{String: deviceID, Valid: true},
			ClientMessageID: sql.NullString{String: input.ClientMessageID, Valid: true},
		})
		if err == nil {
			if err := s.forwardMessage(ctx, userID, existing.ID, input.Sender, input.Body); err != nil {
				return nil, err
			}
			return s.decryptMessage(existing)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	msgID := uuid.New().String()
	senderNonce, senderEnc, err := s.encryptor.Encrypt(input.Sender, fmt.Sprintf("sender:%s", msgID))
	if err != nil {
		applog.Error("service.message", "encrypt sender failed", "message_id", msgID, "error", err)
		return nil, err
	}
	bodyNonce, bodyEnc, err := s.encryptor.Encrypt(input.Body, fmt.Sprintf("body:%s", msgID))
	if err != nil {
		applog.Error("service.message", "encrypt body failed", "message_id", msgID, "error", err)
		return nil, err
	}

	receivedAt := input.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}

	var clientMsgID sql.NullString
	if input.ClientMessageID != "" {
		clientMsgID = sql.NullString{String: input.ClientMessageID, Valid: true}
	}

	row, err := s.queries.CreateMessage(ctx, db.CreateMessageParams{
		ID:              msgID,
		UserID:          userID,
		DeviceID:        sql.NullString{String: deviceID, Valid: true},
		ClientMessageID: clientMsgID,
		SenderNonce:     senderNonce,
		SenderEnc:       senderEnc,
		BodyNonce:       bodyNonce,
		BodyEnc:         bodyEnc,
		KeyVersion:      1,
		ReceivedAt:      receivedAt,
	})
	if err != nil {
		if input.ClientMessageID != "" {
			existing, getErr := s.queries.GetMessageByClientID(ctx, db.GetMessageByClientIDParams{
				UserID:          userID,
				DeviceID:        sql.NullString{String: deviceID, Valid: true},
				ClientMessageID: clientMsgID,
			})
			if getErr == nil {
				if err := s.forwardMessage(ctx, userID, existing.ID, input.Sender, input.Body); err != nil {
					return nil, err
				}
				return s.decryptMessage(existing)
			}
		}
		applog.Error("service.message", "create message failed", "user_id", userID, "device_id", deviceID, "error", err)
		return nil, err
	}

	if err := s.forwardMessage(ctx, userID, msgID, input.Sender, input.Body); err != nil {
		applog.Error("service.message", "forward message failed", "message_id", msgID, "user_id", userID, "error", err)
		return nil, err
	}

	view, err := s.decryptMessage(row)
	if err != nil {
		applog.Error("service.message", "decrypt message failed", "message_id", msgID, "error", err)
		return nil, err
	}

	s.hub.NotifyMessage(userID, map[string]string{"type": "new_message", "id": view.ID})
	return view, nil
}

func (s *MessageService) ensureDevice(ctx context.Context, userID, name string) (string, error) {
	if name == "" {
		name = "Android"
	}
	existing, err := s.queries.GetDeviceByUserAndName(ctx, db.GetDeviceByUserAndNameParams{
		UserID: userID,
		Name:   name,
	})
	if err == nil {
		_ = s.TouchDevice(ctx, userID, existing.ID)
		return existing.ID, nil
	}
	id := uuid.New().String()
	dev, err := s.queries.CreateDevice(ctx, db.CreateDeviceParams{
		ID:       id,
		UserID:   userID,
		Name:     name,
		ClientID: sql.NullString{},
	})
	if err != nil {
		return "", err
	}
	return dev.ID, nil
}

func (s *MessageService) TouchDevice(ctx context.Context, userID, deviceID string) error {
	if _, err := s.queries.GetDeviceByIDAndUser(ctx, db.GetDeviceByIDAndUserParams{
		ID:     deviceID,
		UserID: userID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidDevice
		}
		return err
	}
	now := time.Now().UTC()
	return s.queries.UpdateDeviceLastSeen(ctx, db.UpdateDeviceLastSeenParams{
		LastSeenAt: sql.NullTime{Time: now, Valid: true},
		ID:         deviceID,
	})
}

func (s *MessageService) forwardMessage(ctx context.Context, userID, messageID, sender, body string) error {
	destinations, err := s.queries.ListDestinationsByUser(ctx, userID)
	if err != nil {
		return err
	}
	logs, err := s.queries.ListForwardLogsByMessage(ctx, messageID)
	if err != nil {
		return err
	}

	logByDest := make(map[string]db.ForwardLog, len(logs))
	for _, row := range logs {
		current, ok := logByDest[row.DestinationID]
		if !ok || current.Status != "success" {
			logByDest[row.DestinationID] = row
		}
	}

	for _, dest := range destinations {
		if dest.Enabled != 1 || dest.Platform != "telegram" {
			continue
		}
		forwardLog, ok := logByDest[dest.ID]
		if ok && forwardLog.Status == "success" {
			continue
		}
		if !ok {
			forwardLog, err = s.queries.CreateForwardLog(ctx, db.CreateForwardLogParams{
				ID:            uuid.New().String(),
				MessageID:     messageID,
				DestinationID: dest.ID,
				Status:        "pending",
				Error:         sql.NullString{},
			})
			if err != nil {
				forwardLog, err = s.queries.GetForwardLogByMessageDestination(ctx, db.GetForwardLogByMessageDestinationParams{
					MessageID:     messageID,
					DestinationID: dest.ID,
				})
				if err != nil {
					return err
				}
				if forwardLog.Status == "success" {
					continue
				}
			}
		}

		status := "success"
		var errMsg sql.NullString
		if fwdErr := s.forwardTelegram(ctx, dest, sender, body); fwdErr != nil {
			status = "failed"
			errMsg = sql.NullString{String: fwdErr.Error(), Valid: true}
			applog.Error("service.message", "telegram forward failed",
				"message_id", messageID,
				"destination_id", dest.ID,
				"destination_name", dest.Name,
				"error", fwdErr,
			)
		}
		if err := s.queries.UpdateForwardLog(ctx, db.UpdateForwardLogParams{
			Status: status,
			Error:  errMsg,
			ID:     forwardLog.ID,
		}); err != nil {
			applog.Error("service.message", "forward log update failed",
				"message_id", messageID,
				"destination_id", dest.ID,
				"error", err,
			)
		}
	}
	return nil
}

func (s *MessageService) forwardTelegram(ctx context.Context, dest db.Destination, sender, body string) error {
	cfg, err := s.decryptDestinationConfig(dest)
	if err != nil {
		return err
	}
	text := fmt.Sprintf("📱 新短信\n发件人: %s\n\n%s", sender, body)
	payload := map[string]string{
		"chat_id": cfg.ChatID,
		"text":    text,
	}
	data, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram API status %d", resp.StatusCode)
	}
	return nil
}

func (s *MessageService) decryptDestinationConfig(dest db.Destination) (*TelegramConfig, error) {
	plain, err := s.encryptor.Decrypt(dest.ConfigNonce, dest.ConfigEnc, fmt.Sprintf("config:%s", dest.ID))
	if err != nil {
		return nil, err
	}
	var cfg TelegramConfig
	if err := json.Unmarshal([]byte(plain), &cfg); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.BotToken) == "" || strings.TrimSpace(cfg.ChatID) == "" {
		return nil, fmt.Errorf("invalid telegram config")
	}
	return &cfg, nil
}

func (s *MessageService) ListMessages(ctx context.Context, userID string, limit, offset int64) ([]MessageView, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.queries.ListMessagesByUser(ctx, db.ListMessagesByUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]MessageView, 0, len(rows))
	for _, row := range rows {
		view, err := s.decryptMessage(row)
		if err != nil {
			continue
		}
		out = append(out, *view)
	}
	return out, nil
}

func (s *MessageService) decryptMessage(row db.InboundMessage) (*MessageView, error) {
	sender, err := s.encryptor.Decrypt(row.SenderNonce, row.SenderEnc, fmt.Sprintf("sender:%s", row.ID))
	if err != nil {
		return nil, err
	}
	body, err := s.encryptor.Decrypt(row.BodyNonce, row.BodyEnc, fmt.Sprintf("body:%s", row.ID))
	if err != nil {
		return nil, err
	}
	return &MessageView{
		ID:         row.ID,
		Sender:     sender,
		Body:       body,
		ReceivedAt: row.ReceivedAt,
		CreatedAt:  row.CreatedAt,
	}, nil
}

func EncryptDestinationConfig(encryptor *crypto.FieldEncryptor, destID string, cfg TelegramConfig) (nonce, ciphertext []byte, err error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, nil, err
	}
	return encryptor.Encrypt(string(data), fmt.Sprintf("config:%s", destID))
}

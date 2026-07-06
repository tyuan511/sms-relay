package telegram

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/sms-relay/server/internal/applog"
)

const LinkTTL = 5 * time.Minute

var (
	ErrLinkNotFound  = errors.New("link session not found")
	ErrLinkExpired   = errors.New("link session expired")
	ErrLinkForbidden = errors.New("link session forbidden")
)

type LinkStatus string

const (
	LinkStatusPending LinkStatus = "pending"
	LinkStatusLinked  LinkStatus = "linked"
	LinkStatusExpired LinkStatus = "expired"
	LinkStatusFailed  LinkStatus = "failed"
)

type LinkSession struct {
	ID            string
	UserID        string
	Name          string
	BotToken      string
	LinkSecret    string
	BotUsername   string
	StartURL      string
	ExpiresAt     time.Time
	Status        LinkStatus
	ChatID        string
	DestinationID string
	Error         string
	UpdateOffset  int
	completing    bool
}

type Linker struct {
	client   *Client
	mu       sync.RWMutex
	byID     map[string]*LinkSession
	pollLock map[string]*sync.Mutex // keyed by bot token
}

func NewLinker(client *Client) *Linker {
	return &Linker{
		client:   client,
		byID:     make(map[string]*LinkSession),
		pollLock: make(map[string]*sync.Mutex),
	}
}

func (l *Linker) Start(ctx context.Context, userID, name, botToken string) (*LinkSession, error) {
	bot, err := l.client.GetMe(ctx, botToken)
	if err != nil {
		applog.Error("telegram.linker", "start link getMe failed", "user_id", userID, "error", err)
		return nil, err
	}
	if err := l.client.PreparePolling(ctx, botToken); err != nil {
		applog.Error("telegram.linker", "start link prepare polling failed", "user_id", userID, "bot_username", bot.Username, "error", err)
		return nil, err
	}

	offset, err := l.client.LatestUpdateOffset(ctx, botToken)
	if err != nil {
		applog.Error("telegram.linker", "start link get update offset failed", "user_id", userID, "bot_username", bot.Username, "error", err)
		return nil, err
	}

	secret, err := randomSecret()
	if err != nil {
		return nil, err
	}

	session := &LinkSession{
		ID:           newID(),
		UserID:       userID,
		Name:         name,
		BotToken:     botToken,
		LinkSecret:   secret,
		BotUsername:  bot.Username,
		StartURL:     "https://t.me/" + bot.Username + "?start=" + secret,
		ExpiresAt:    time.Now().Add(LinkTTL),
		Status:       LinkStatusPending,
		UpdateOffset: offset,
	}

	l.mu.Lock()
	l.cancelPendingForToken(botToken, session.ID)
	l.byID[session.ID] = session
	l.mu.Unlock()

	return l.copySession(session), nil
}

func (l *Linker) cancelPendingForToken(botToken, exceptID string) {
	for id, s := range l.byID {
		if s.BotToken == botToken && s.Status == LinkStatusPending && id != exceptID {
			s.Status = LinkStatusFailed
			s.Error = "已被新的绑定请求取代"
		}
	}
}

func (l *Linker) Get(linkID, userID string) (*LinkSession, error) {
	l.mu.RLock()
	session, ok := l.byID[linkID]
	l.mu.RUnlock()
	if !ok {
		return nil, ErrLinkNotFound
	}
	if session.UserID != userID {
		return nil, ErrLinkForbidden
	}
	if session.Status == LinkStatusPending && time.Now().After(session.ExpiresAt) {
		l.markExpired(linkID)
		session.Status = LinkStatusExpired
	}
	return l.copySession(session), nil
}

func (l *Linker) Poll(ctx context.Context, linkID string) error {
	l.mu.RLock()
	session, ok := l.byID[linkID]
	if !ok {
		l.mu.RUnlock()
		return ErrLinkNotFound
	}
	if session.Status != LinkStatusPending || time.Now().After(session.ExpiresAt) {
		l.mu.RUnlock()
		return nil
	}
	token := session.BotToken
	l.mu.RUnlock()

	lock := l.tokenPollLock(token)
	lock.Lock()
	defer lock.Unlock()

	l.mu.RLock()
	session, ok = l.byID[linkID]
	if !ok || session.Status != LinkStatusPending {
		l.mu.RUnlock()
		return nil
	}
	offset := session.UpdateOffset
	secret := session.LinkSecret
	l.mu.RUnlock()

	updates, err := l.client.GetUpdates(ctx, token, offset, 5)
	if err != nil {
		applog.Warn("telegram.linker", "link poll failed", "link_id", linkID, "error", err)
		return err
	}

	for _, upd := range updates {
		if upd.UpdateID >= offset {
			offset = upd.UpdateID + 1
		}
		if upd.Message == nil {
			continue
		}
		if !ShouldBindMessage(upd.Message.Text, secret, upd.Message.Chat.Type) {
			continue
		}

		l.mu.Lock()
		if s, ok := l.byID[linkID]; ok && s.Status == LinkStatusPending {
			s.ChatID = FormatChatID(upd.Message.Chat.ID)
			s.UpdateOffset = offset
		}
		l.mu.Unlock()
		return nil
	}

	l.mu.Lock()
	if s, ok := l.byID[linkID]; ok {
		s.UpdateOffset = offset
	}
	l.mu.Unlock()
	return nil
}

func (l *Linker) tokenPollLock(token string) *sync.Mutex {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.pollLock[token] == nil {
		l.pollLock[token] = &sync.Mutex{}
	}
	return l.pollLock[token]
}

func (l *Linker) Complete(linkID, userID, destinationID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	session, ok := l.byID[linkID]
	if !ok || session.UserID != userID {
		return
	}
	session.Status = LinkStatusLinked
	session.DestinationID = destinationID
}

func (l *Linker) Fail(linkID, userID, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	session, ok := l.byID[linkID]
	if !ok || session.UserID != userID {
		return
	}
	session.Status = LinkStatusFailed
	session.Error = message
	session.completing = false
}

func (l *Linker) Remove(linkID string) {
	l.mu.Lock()
	delete(l.byID, linkID)
	l.mu.Unlock()
}

func (l *Linker) markExpired(linkID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if session, ok := l.byID[linkID]; ok && session.Status == LinkStatusPending {
		session.Status = LinkStatusExpired
	}
}

func (l *Linker) copySession(s *LinkSession) *LinkSession {
	cp := *s
	return &cp
}

func (l *Linker) BeginComplete(linkID, userID string) (*LinkSession, string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	session, ok := l.byID[linkID]
	if !ok || session.UserID != userID {
		return nil, "", false
	}
	if session.Status == LinkStatusLinked {
		return session, "", false
	}
	if session.ChatID == "" || session.completing {
		return session, "", false
	}
	session.completing = true
	return session, session.ChatID, true
}

func randomSecret() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

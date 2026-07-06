package applog

import (
	"log/slog"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func Init() {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

func Debug(component, msg string, kv ...any) {
	slog.Debug(msg, append([]any{slog.String("component", component)}, kv...)...)
}

func Info(component, msg string, kv ...any) {
	slog.Info(msg, append([]any{slog.String("component", component)}, kv...)...)
}

func Warn(component, msg string, kv ...any) {
	slog.Warn(msg, append([]any{slog.String("component", component)}, kv...)...)
}

func Error(component, msg string, kv ...any) {
	slog.Error(msg, append([]any{slog.String("component", component)}, kv...)...)
}

func ReqError(component, operation string, c *fiber.Ctx, status int, err error, kv ...any) {
	args := []any{
		slog.String("operation", operation),
		slog.String("method", c.Method()),
		slog.String("path", c.Path()),
		slog.Int("status", status),
	}
	if uid, ok := c.Locals("userID").(string); ok && uid != "" {
		args = append(args, slog.String("user_id", uid))
	}
	if did, ok := c.Locals("deviceID").(string); ok && did != "" {
		args = append(args, slog.String("device_id", did))
	}
	if err != nil {
		args = append(args, slog.String("error", err.Error()))
	}
	args = append(args, kv...)
	Error(component, "request failed", args...)
}

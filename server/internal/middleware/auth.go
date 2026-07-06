package middleware

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/sms-relay/server/internal/applog"
	"github.com/sms-relay/server/internal/auth"
)

var (
	errMissingAuth = errors.New("missing authorization")
	errInvalidAuth = errors.New("invalid token")
)

func JWTAuth(secret string) fiber.Handler {
	return UserJWTAuth(secret)
}

func UserJWTAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := parseBearer(c.Get("Authorization"), secret)
		if err != nil {
			applog.Warn("auth", "user authentication failed",
				"method", c.Method(),
				"path", c.Path(),
				"reason", err.Error(),
			)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"detail": err.Error()})
		}
		if !claims.IsUser() {
			applog.Error("auth", "user token required",
				"method", c.Method(),
				"path", c.Path(),
				"token_type", claims.Type,
				"user_id", claims.UserID,
				"device_id", claims.DeviceID,
			)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"detail": "user token required"})
		}
		c.Locals("userID", claims.UserID)
		return c.Next()
	}
}

func DeviceJWTAuth(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := parseBearer(c.Get("Authorization"), secret)
		if err != nil {
			applog.Warn("auth", "device authentication failed",
				"method", c.Method(),
				"path", c.Path(),
				"reason", err.Error(),
			)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"detail": err.Error()})
		}
		if !claims.IsDevice() {
			applog.Error("auth", "device token required",
				"method", c.Method(),
				"path", c.Path(),
				"token_type", claims.Type,
				"user_id", claims.UserID,
			)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"detail": "device token required"})
		}
		c.Locals("userID", claims.UserID)
		c.Locals("deviceID", claims.DeviceID)
		return c.Next()
	}
}

func parseBearer(header, secret string) (*auth.Claims, error) {
	if !strings.HasPrefix(header, "Bearer ") {
		return nil, errMissingAuth
	}
	token := strings.TrimPrefix(header, "Bearer ")
	claims, err := auth.ParseToken(token, secret)
	if err != nil {
		return nil, errInvalidAuth
	}
	return claims, nil
}

func UserID(c *fiber.Ctx) string {
	v, _ := c.Locals("userID").(string)
	return v
}

func DeviceID(c *fiber.Ctx) string {
	v, _ := c.Locals("deviceID").(string)
	return v
}

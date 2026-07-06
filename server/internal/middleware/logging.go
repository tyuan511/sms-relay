package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/sms-relay/server/internal/applog"
)

func HTTPErrorLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()
		status := c.Response().StatusCode()
		if status < 400 {
			return err
		}
		if err != nil {
			applog.Error("http", "response error",
				"method", c.Method(),
				"path", c.Path(),
				"status", status,
				"error", err.Error(),
			)
			return err
		}
		applog.Warn("http", "error response",
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
		)
		return nil
	}
}

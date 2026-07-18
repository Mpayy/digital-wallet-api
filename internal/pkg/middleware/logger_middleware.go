package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func LoggerMiddleware(log *logrus.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		path := ctx.Request.URL.Path
		raw := ctx.Request.URL.RawQuery

		ctx.Next()

		latency := time.Since(start)
		status := ctx.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		fields := logrus.Fields{
			"method":     ctx.Request.Method,
			"path":       path,
			"status":     status,
			"latency_ms": latency.Milliseconds(),
			"client_ip":  ctx.ClientIP(),
		}

		errCode, ok := ctx.Get("error_code")
		if ok {
			fields["error_code"] = errCode
		}

		if len(ctx.Errors) > 0 {
			fields["error"] = ctx.Errors.Last().Err.Error() // detail akar masalah, kalau ada
		}

		entry := log.WithFields(fields)

		switch {
		case status >= 500:
			entry.Error("server error")
		case status >= 400:
			entry.Warn("client error")
		default:
			entry.Info("request handled")
		}
	}
}

package logging

import (
	"bytes"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	EmojiStartup  = "🚀"
	EmojiShutdown = "🛑"
	EmojiRequest  = "📥"
	EmojiResponse = "📤"
	EmojiSuccess  = "✅"
	EmojiError    = "❌"
	EmojiWarning  = "⚠️"
	EmojiAuth     = "🔐"
	EmojiUser     = "👤"
	EmojiDatabase = "🗄️"
	EmojiNetwork  = "🌐"
	EmojiMetrics  = "📊"
	EmojiHealth   = "💚"
	EmojiConfig   = "⚙️"
	EmojiOAuth    = "🔑"
	EmojiEmail    = "📧"
	EmojiApple    = "🍎"
	EmojiGoogle   = "🔷"
)

type Logger struct {
	*zap.Logger
}

func New(level string, isProduction bool) (*Logger, error) {
	var config zap.Config

	if isProduction {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, err := config.Build(
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, err
	}

	return &Logger{Logger: logger}, nil
}

func (l *Logger) WithEmoji(emoji string, msg string) string {
	return emoji + " " + msg
}

func (l *Logger) Startup(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiStartup, msg), fields...)
}

func (l *Logger) Shutdown(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiShutdown, msg), fields...)
}

func (l *Logger) Request(msg string, fields ...zap.Field) {
	l.Logger.Debug(l.WithEmoji(EmojiRequest, msg), fields...)
}

func (l *Logger) Response(msg string, fields ...zap.Field) {
	l.Logger.Debug(l.WithEmoji(EmojiResponse, msg), fields...)
}

func (l *Logger) AuthSuccess(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiSuccess+" "+EmojiAuth, msg), fields...)
}

func (l *Logger) AuthError(msg string, fields ...zap.Field) {
	l.Logger.Error(l.WithEmoji(EmojiError+" "+EmojiAuth, msg), fields...)
}

func (l *Logger) AuthWarning(msg string, fields ...zap.Field) {
	l.Logger.Warn(l.WithEmoji(EmojiWarning+" "+EmojiAuth, msg), fields...)
}

func (l *Logger) EmailAuth(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiEmail, msg), fields...)
}

func (l *Logger) AppleAuth(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiApple, msg), fields...)
}

func (l *Logger) GoogleAuth(msg string, fields ...zap.Field) {
	l.Logger.Info(l.WithEmoji(EmojiGoogle, msg), fields...)
}

// OAuthSuccess logs a successful OAuth authentication with masked user info.
func (l *Logger) OAuthSuccess(provider string, email string, userID string, fields ...zap.Field) {
	emoji := EmojiOAuth
	switch strings.ToLower(provider) {
	case "apple":
		emoji = EmojiApple
	case "google":
		emoji = EmojiGoogle
	}

	baseFields := []zap.Field{
		zap.String("provider", provider),
		zap.String("email", MaskEmail(email)),
		zap.String("user_id", MaskUserID(userID)),
	}
	allFields := append(baseFields, fields...)

	l.Logger.Info(l.WithEmoji(emoji+" "+EmojiSuccess, "user authenticated"), allFields...)
}

func (l *Logger) Health(msg string, fields ...zap.Field) {
	l.Logger.Debug(l.WithEmoji(EmojiHealth, msg), fields...)
}

func (l *Logger) NetworkError(msg string, fields ...zap.Field) {
	l.Logger.Error(l.WithEmoji(EmojiNetwork+" "+EmojiError, msg), fields...)
}

func (l *Logger) DatabaseError(msg string, fields ...zap.Field) {
	l.Logger.Error(l.WithEmoji(EmojiDatabase+" "+EmojiError, msg), fields...)
}

func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

func Must(level string, isProduction bool) *Logger {
	logger, err := New(level, isProduction)
	if err != nil {
		os.Exit(1)
	}
	return logger
}

// Masking functions for sensitive data

// MaskEmail masks an email address, showing only the first 2 characters and domain.
// Example: "john.doe@example.com" -> "jo***@example.com"
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}

	local := parts[0]
	domain := parts[1]

	if len(local) <= 2 {
		return local + "***@" + domain
	}

	return local[:2] + "***@" + domain
}

// MaskUserID masks a user ID, showing only the first 8 characters.
// Example: "550e8400-e29b-41d4-a716-446655440000" -> "550e8400-****"
func MaskUserID(userID string) string {
	if userID == "" {
		return ""
	}

	if len(userID) <= 8 {
		return userID + "-****"
	}

	return userID[:8] + "-****"
}

// SensitiveFields are field names that should not be logged.
var SensitiveFields = []string{
	"password",
	"access_token",
	"refresh_token",
	"token",
	"apikey",
	"secret",
	"id_token",
	"provider_token",
	"provider_refresh_token",
}

// SanitizeBody removes sensitive fields from JSON bodies for logging.
func SanitizeBody(body []byte) string {
	const maxLen = 1024
	s := string(body)
	if len(s) > maxLen {
		return s[:maxLen] + "...(truncated)"
	}

	// Check for sensitive field patterns
	for _, field := range SensitiveFields {
		pattern := `"` + field + `"`
		if bytes.Contains(body, []byte(pattern)) {
			return "[body contains sensitive data - not logged]"
		}
	}

	return s
}

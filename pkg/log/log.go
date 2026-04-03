package log

import "go.uber.org/zap"

var logger *zap.Logger

func Init(service string, level string) {
	config := zap.NewProductionConfig()
	config.InitialFields = map[string]interface{}{
		"service": service,
	}
	logger, _ = config.Build()
}

func Debug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	logger.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	logger.Fatal(msg, fields...)
}

func WithField(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}

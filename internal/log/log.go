package log

import "github.com/teachain/deepseek/log"

var logger = log.New()

func Init(service string, level string) {
	logger = log.New(
		log.WithServiceName(service),
		log.WithLevel(level),
	)
}

func Debug(msg string, fields ...log.Field) {
	logger.Debug(msg, fields...)
}

func Info(msg string, fields ...log.Field) {
	logger.Info(msg, fields...)
}

func Warn(msg string, fields ...log.Field) {
	logger.Warn(msg, fields...)
}

func Error(msg string, fields ...log.Field) {
	logger.Error(msg, fields...)
}

func Fatal(msg string, fields ...log.Field) {
	logger.Fatal(msg, fields...)
}

func WithField(key string, value interface{}) log.Field {
	return log.WithField(key, value)
}

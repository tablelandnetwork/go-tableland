package logging

import (
	"os"
	"runtime"
	"time"

	"cloud.google.com/go/logging"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// SetupLogger configures the logging library.
func SetupLogger(version string, debug, human bool) {
	zerolog.TimestampFieldName = "timestamp"
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if human {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	log.Logger = log.Logger.Hook(googleSeverityHook{})
	log.Logger = log.With().
		Str("version", version).
		Str("goversion", runtime.Version()).
		Logger()
}

type googleSeverityHook struct{}

func (h googleSeverityHook) Run(e *zerolog.Event, level zerolog.Level, _ string) {
	e.Str("severity", levelToSeverity(level).String())
}

// converts zerolog level to google's severity.
func levelToSeverity(level zerolog.Level) logging.Severity {
	switch level {
	case zerolog.DebugLevel:
		return logging.Debug
	case zerolog.WarnLevel:
		return logging.Warning
	case zerolog.ErrorLevel:
		return logging.Error
	case zerolog.FatalLevel:
		return logging.Alert
	case zerolog.PanicLevel:
		return logging.Emergency
	default:
		return logging.Info
	}
}

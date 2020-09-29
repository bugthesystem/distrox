package common

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
)

type Logger interface {
	Printf(format string, v ...interface{})
	Err(msg string, err error)
	Fatal(msg string, err error)
	Debug(msg string)
	Info(msg string)
}

type NilLogger struct {
}

func (NilLogger) Info(msg string) {
	// no op
}

func (NilLogger) Debug(_ string) {
	// no op
}

func (NilLogger) Err(_ string, _ error) {
	// no op
}

func (NilLogger) Fatal(_ string, _ error) {
	// no op
}

func (NilLogger) Printf(_ string, _ ...interface{}) {
	//no op
}

func NewDefaultLogger() Logger {
	return &NilLogger{}
}

type ZeroLogger struct {
	log zerolog.Logger
}

func (l *ZeroLogger) Info(msg string) {
	l.log.Info().Msg(msg)
}

func (l *ZeroLogger) Debug(msg string) {
	l.log.Debug().Msg(msg)
}

func (l *ZeroLogger) Fatal(msg string, err error) {
	l.log.Fatal().Err(err).Msg(msg)
}

func (l *ZeroLogger) Printf(format string, v ...interface{}) {
	l.log.Printf(format, v...)
}

func (l *ZeroLogger) Err(msg string, err error) {
	l.log.Err(err).Msg(msg)
}

// newZeroLogger returns a `Logger` implementation
// backed by zerolog's log
func NewZeroLogger(mode string) Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if mode == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	return &ZeroLogger{
		log: zerolog.New(diode.NewWriter(os.Stdout, 10000, 10*time.Millisecond, func(missed int) {
			fmt.Printf("Logger Dropped %d messages", missed)
		})),
	}
}

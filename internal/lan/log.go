package lan

import (
	"github.com/pion/logging"
	"github.com/rs/zerolog"
)

// zerologFactory routes pion's leveled logging into zerolog. Trace output
// is dropped: it logs every packet.
type zerologFactory struct{ log *zerolog.Logger }

func (f zerologFactory) NewLogger(scope string) logging.LeveledLogger {
	return zerologLogger{f.log.With().Str("component", "mdns").Str("scope", scope).Logger()}
}

type zerologLogger struct{ log zerolog.Logger }

func (zerologLogger) Trace(string)          {}
func (zerologLogger) Tracef(string, ...any) {}

func (l zerologLogger) Debug(msg string)                  { l.log.Debug().Msg(msg) }
func (l zerologLogger) Debugf(format string, args ...any) { l.log.Debug().Msgf(format, args...) }
func (l zerologLogger) Info(msg string)                   { l.log.Info().Msg(msg) }
func (l zerologLogger) Infof(format string, args ...any)  { l.log.Info().Msgf(format, args...) }
func (l zerologLogger) Warn(msg string)                   { l.log.Warn().Msg(msg) }

// parseFailureFormat is pion's warning for a packet it could not fully
// parse. pion/mdns v2.0.7 skips non-A/AAAA answers without consuming them,
// so other devices' service announcements (PTR, SRV, TXT) trip it on every
// broadcast. Questions are handled before answers are parsed, so the
// failure never affects arrd's own answers: log it at debug only.
const parseFailureFormat = "[%s] failed to parse mDNS packet %v"

func (l zerologLogger) Warnf(format string, args ...any) {
	if format == parseFailureFormat {
		l.log.Debug().Msgf(format, args...)
		return
	}
	l.log.Warn().Msgf(format, args...)
}

func (l zerologLogger) Error(msg string)                  { l.log.Error().Msg(msg) }
func (l zerologLogger) Errorf(format string, args ...any) { l.log.Error().Msgf(format, args...) }

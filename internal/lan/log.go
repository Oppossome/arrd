package lan

import (
	"fmt"
	"log/slog"

	"github.com/pion/logging"
)

// slogFactory routes pion's leveled logging into slog. Trace output is
// dropped: it logs every packet.
type slogFactory struct{ log *slog.Logger }

func (f slogFactory) NewLogger(scope string) logging.LeveledLogger {
	return slogLogger{f.log.With("component", "mdns", "scope", scope)}
}

type slogLogger struct{ log *slog.Logger }

func (slogLogger) Trace(string)          {}
func (slogLogger) Tracef(string, ...any) {}

func (l slogLogger) Debug(msg string)                  { l.log.Debug(msg) }
func (l slogLogger) Debugf(format string, args ...any) { l.log.Debug(fmt.Sprintf(format, args...)) }
func (l slogLogger) Info(msg string)                   { l.log.Info(msg) }
func (l slogLogger) Infof(format string, args ...any)  { l.log.Info(fmt.Sprintf(format, args...)) }
func (l slogLogger) Warn(msg string)                   { l.log.Warn(msg) }
func (l slogLogger) Warnf(format string, args ...any)  { l.log.Warn(fmt.Sprintf(format, args...)) }
func (l slogLogger) Error(msg string)                  { l.log.Error(msg) }
func (l slogLogger) Errorf(format string, args ...any) { l.log.Error(fmt.Sprintf(format, args...)) }

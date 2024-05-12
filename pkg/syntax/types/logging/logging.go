package logging

import (
	"io"
	"log/slog"
	"strings"

	"github.com/henderiw/logger/log"
)

// PluginOutputMonitor creates an io.Writer that will warn about any writes in
// the default logger. This is used to catch unexpected output from plugins,
// notifying them about the problem as well as surfacing the lost data for
// context.
func PluginOutputMonitor(source string) io.Writer {
	return pluginOutputMonitor{
		source: source,
		log:    log.NewLogger(&log.HandlerOptions{Name: "kform-plugin-logger", AddSource: false}),
	}
}

// pluginOutputMonitor is an io.Writer that logs all writes as
// "unexpected data" with the source name.
type pluginOutputMonitor struct {
	source string
	log    *slog.Logger
}

func (w pluginOutputMonitor) Write(d []byte) (int, error) {
	// Limit the write size to 1024 bytes We're not expecting any data to come
	// through this channel, so accidental writes will usually be stray fmt
	// debugging statements and the like, but we want to provide context to the
	// provider to indicate what the unexpected data might be.
	n := len(d)
	if n > 1024 {
		d = append(d[:1024], '.', '.', '.')
	}

	w.log.Warn("unexpected data", w.source, strings.TrimSpace(string(d)))
	return n, nil
}

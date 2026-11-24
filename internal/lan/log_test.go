package lan

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestParseFailureLoggedAtDebug(t *testing.T) {
	var buf bytes.Buffer
	log := zerologFactory{new(zerolog.New(&buf).Level(zerolog.InfoLevel))}.NewLogger("mdns")

	log.Warnf(parseFailureFormat, "arrd", errors.New("parsing/packing of this type isn't available yet"))
	if buf.Len() != 0 {
		t.Errorf("parse failure logged at info level: %s", buf.String())
	}

	log.Warnf("[%s] failed to send mDNS packet on interface %d: %v", "arrd", 2, errors.New("network is unreachable"))
	if !strings.Contains(buf.String(), `"level":"warn"`) {
		t.Errorf("other warnings suppressed: %q", buf.String())
	}
}

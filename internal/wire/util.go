package wire

import (
	"fmt"
	"strconv"
)

func fmtSscanf(hhmm string, h, m *int) (int, error) { return fmt.Sscanf(hhmm, "%d:%d", h, m) }
func itoa(n int) string                             { return strconv.Itoa(n) }
func pad(n int) string                              { return fmt.Sprintf("%02d", n) }

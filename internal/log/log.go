package log

import (
	"fmt"
	"io"
	"strconv"
	"time"
)

type Prefix [2]byte

var (
	TT = Prefix{'T', ':'}
	DD = Prefix{'D', ':'}
	II = Prefix{'I', ':'}
	WW = Prefix{'W', ':'}
	EE = Prefix{'E', ':'}
	FF = Prefix{'F', ':'}

	N0 = Prefix{'0', '<'}
	N1 = Prefix{'1', '>'}
	N2 = Prefix{'2', '>'}
	N3 = Prefix{'3', '>'}
	N4 = Prefix{'4', '>'}
	N5 = Prefix{'5', '>'}
	N6 = Prefix{'6', '>'}
	N7 = Prefix{'7', '>'}
	N8 = Prefix{'8', '>'}
	N9 = Prefix{'9', '>'}

	sp = []byte(" ")
	nl = []byte("\n")

	t = time.Now()
)

func Uptime() time.Duration {
	return time.Since(t)
}

func format(c [2]byte, d []byte, s string) string {
	return fmt.Sprintf("%s %s %s\n", c, d, s)
}

// FormatDuration fixed-width formats duration with best-fit precision.
func FormatDuration(d time.Duration) []byte {
	dur := [15]byte{'0', '0', '0', '0', '0', '0', '0', '0', '0', '0', '0', '0', '0', '0', 's'}
	dot := 9
	switch {
	case d < 10*time.Second:
		dot--
		fallthrough
	case d < time.Minute:
		dot -= 2
		fallthrough
	case d < 10*time.Minute:
		dot--
		fallthrough
	case d < time.Hour:
		dot -= 2
		fallthrough
	case d < 10*time.Hour:
		dot--
		fallthrough
	case d < 100*time.Hour:
		dot--
	case d < 1000*time.Hour:
	default:
		d %= 1000 * time.Hour
	}
	dur[dot] = '.'
	switch {
	case d >= time.Hour:
		buf := strconv.AppendInt(nil, int64(d/time.Hour), 10)
		copy(dur[dot-6-len(buf):dot-6], buf)
		d %= time.Hour
		dur[dot-6] = 'h'
		fallthrough
	case d >= time.Minute:
		buf := strconv.AppendInt(nil, int64(d/time.Minute), 10)
		copy(dur[dot-3-len(buf):dot-3], buf)
		d %= time.Minute
		dur[dot-3] = 'm'
		fallthrough
	case d >= time.Second:
		buf := strconv.AppendInt(nil, int64(d/time.Second), 10)
		copy(dur[dot-len(buf):dot], buf)
		d %= time.Second
		fallthrough
	default:
		buf := strconv.AppendInt(nil, int64(d), 10)
		copy(dur[dot+1:len(dur)-1], buf)
	}
	return dur[:]
}

func Errorf(w io.Writer, s string, a ...interface{}) {
	if len(a) == 0 {
		Logf(w, EE, s, a...)
	} else if err, ok := a[0].(error); a[0] != nil && (!ok || err != nil) {
		Logf(w, EE, s, a...)
	}
}

func Debugf(w io.Writer, s string, a ...interface{}) {
	Logf(w, DD, s, a...)
}

func Infof(w io.Writer, s string, a ...interface{}) {
	Logf(w, II, s, a...)
}

func Warnf(w io.Writer, s string, a ...interface{}) {
	Logf(w, WW, s, a...)
}

func Logf(w io.Writer, p Prefix, s string, a ...interface{}) {
	fmt.Fprintf(w, format(p, FormatDuration(Uptime()), s), a...)
}

func Log(w io.Writer, p Prefix, d time.Duration, bs []byte) {
	_, _ = w.Write(p[:])
	_, _ = w.Write(sp)
	_, _ = w.Write(FormatDuration(d))
	_, _ = w.Write(sp)
	_, _ = w.Write(bs)
	_, _ = w.Write(nl)
}

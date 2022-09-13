package log

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"time"
)

type Prefix [2]byte

var (
	TT = Prefix{'T', 'T'}
	DD = Prefix{'D', 'D'}
	II = Prefix{'I', 'I'}
	WW = Prefix{'W', 'W'}
	EE = Prefix{'E', 'E'}
	FF = Prefix{'F', 'F'}

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

	Line = Prefix{}
	More = []byte(nil)

	t = time.Now()
)

// uptime fixed-width-format uptime with best-fit precision.
func uptime() []byte {
	const maxDuration = 1000000 * time.Hour
	d := time.Since(t)
	if d >= maxDuration {
		d -= maxDuration
	}
	var bs [15]byte
	buf := bs[:0]
	switch {
	case d >= time.Hour:
		buf = strconv.AppendInt(buf, int64(d/time.Hour), 10)
		buf = append(buf, 'h')
		d %= time.Hour
		fallthrough
	case d >= time.Minute:
		if len(buf) != 0 && d < time.Minute*10 {
			buf = append(buf, '0')
		}
		buf = strconv.AppendInt(buf, int64(d/time.Minute), 10)
		buf = append(buf, 'm')
		d %= time.Minute
		fallthrough
	default:
		prec := cap(buf) - len(buf) - 4
		if d < time.Second*10 {
			if len(buf) != 0 {
				buf = append(buf, '0')
			} else {
				prec++
			}
		}
		buf = strconv.AppendFloat(buf, d.Seconds(), 'f', prec, 64)
		buf = append(buf, 's')
	}
	return buf
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
	var buf bytes.Buffer
	_, _ = fmt.Fprintf(&buf, s, a...)
	Logv(w, p, buf.Bytes())
}

func Logv(w io.Writer, p Prefix, iov ...[]byte) {
	var buf = [20 + 1004]byte{
		p[0], p[1], ' ', '0', '.', '0', '0', '0', '0', '0', '0', '0', '0', '0', '0', '0', '0', 's', ' ', '\n',
	}
	i := 19
	if p != Line {
		copy(buf[3:18], uptime())
		i = 0
	}
	if len(iov) == 0 {
		_, _ = w.Write(buf[i:20])
		return
	}
	if len(iov) == 1 && len(iov[0]) != 0 && 19+len(iov[0]) < len(buf) {
		copy(buf[19:], iov[0])
		buf[19+len(iov[0])] = '\n'
		_, _ = w.Write(buf[i : 19+len(iov[0])+1])
		return
	}
	if i != 19 {
		_, _ = w.Write(buf[i:19])
	}
	for _, bs := range iov {
		if len(bs) != 0 {
			_, _ = w.Write(bs)
		}
	}
	if iov[len(iov)-1] != nil {
		_, _ = w.Write(buf[19:20])
	}
}

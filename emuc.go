package emucb201

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Port uint8

const (
	PORT1  Port = 1
	PORT2  Port = 2
	PORT12 Port = 3
)

func (p Port) String() string {
	switch p {
	case PORT1:
		return "1"
	case PORT2:
		return "2"
	case PORT12:
		return "12"
	}
	return strconv.Itoa(int(p))
}

type CanMsg struct {
	Flags  uint8   // bit 0: Ext, bit 1: RTR bit 2:6 DLC (0..8)
	Header uint32  // CAN address 10 or 29 bit depending on Flags & 1
	Buf    [8]byte // zero padded
}

func (msg *CanMsg) IsExt() bool     { return (msg.Flags & 1) != 0 }
func (msg *CanMsg) IsRTR() bool     { return (msg.Flags & 2) != 0 }
func (msg *CanMsg) SetRTR()         { msg.Flags |= 2 }
func (msg *CanMsg) DLC() int        { return int(msg.Flags >> 2) }
func (msg *CanMsg) Payload() []byte { return msg.Buf[:msg.DLC()] }

func (msg *CanMsg) String() string {
	if msg.IsExt() {
		return fmt.Sprintf("%03x:% x", msg.Header, msg.Buf[:msg.DLC()])
	}
	return fmt.Sprintf("%08x:% x", msg.Header, msg.Buf[:msg.DLC()])
}

func NewMessage(header uint32, payload []byte) *CanMsg {
	r := &CanMsg{Flags: uint8(len(payload) << 2), Header: header & ((1 << 10) - 1)}
	copy(r.Buf[:0], payload)
	return r
}

func NewExtMessage(header uint32, payload []byte) *CanMsg {
	r := &CanMsg{Flags: uint8(len(payload)<<2) | 1, Header: header & ((1 << 29) - 1)}
	copy(r.Buf[:0], payload)
	return r
}

func Encode(w io.Writer, port Port, msg *CanMsg) error {
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, port)
	binary.Write(&b, binary.BigEndian, msg)
	if b.Len() != 15 {
		return fmt.Errorf("expected 15 bytes after encoding CanMsg, got %d", b.Len())
	}

	var buf bytes.Buffer
	buf.WriteByte('<')
	buf.WriteString(strings.ToUpper(hex.EncodeToString(b.Bytes())))
	chk := uint8(0xff)
	for _, v := range b.Bytes() {
		chk += v
	}
	buf.WriteString(strings.ToUpper(hex.EncodeToString([]byte{chk})))
	buf.WriteString("\r'n")
	_, err := w.Write(buf.Bytes())
	return err
}

type Decoder struct{ *bufio.Reader }

var (
	ErrNak = errors.New("BaudRate set NAK message")
)

func NewDecoder(r io.Reader) Decoder { return Decoder{bufio.NewReader(r)} }

// Decode returns the next Can Message decoded from the stream.
// Decode handles 2 special cases: ack (";019B") and nack (";009A")
// which are returned in response to setting a baudrate.
// An Ack message returns 0,nil,nil, an NAK message returns 0,nil,ErrNak
func (d Decoder) Decode() (port Port, msg *CanMsg, err error) {
	line, err := d.ReadString('\n')
	if err != nil {
		return 0, nil, err
	}
	switch {
	case strings.HasPrefix(line, ";009A"):
		return 0, nil, ErrNak
	case strings.HasPrefix(line, ";019B"):
		return 0, nil, nil
	case line[0] == '=':
		break
	default:
		return 0, nil, fmt.Errorf("Invalid message %q", line)
	}

	buf, err := hex.DecodeString(line[1:])
	if err != nil {
		return 0, nil, err
	}
	if len(buf) != 15 {
		return 0, nil, fmt.Errorf("Expected 30 hex digits, got %q", line)
	}

	chk := uint8(0xff)
	for _, v := range line[:29] {
		chk += uint8(v)
	}
	if chk != buf[len(buf)-1] {
		return 0, nil, fmt.Errorf("bad checksum: %v != %v", chk, buf[14])
	}

	err = binary.Read(bytes.NewReader(buf[1:14]), binary.BigEndian, msg)
	return Port(buf[0]), msg, err

}

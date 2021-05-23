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

// A Port represents which of the two (or both) CAN ports a message is received or transmitted on.
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

// A CanMsg represents a received or transmitted message over either or both ports.
type CanMsg struct {
	Flags  uint8   // bit 0: Ext, bit 1: RTR bit 2:6 DLC (0..8)
	Header uint32  // CAN address 10 or 29 bit depending on Flags & 1
	Buf    [8]byte // zero padded
}

// IsExt is true iff the address is 29 bits
func (msg *CanMsg) IsExt() bool { return (msg.Flags & 1) != 0 }

// IsRTR indicates if the RTR bit is set.  nobody uses this anymore.
func (msg *CanMsg) IsRTR() bool { return (msg.Flags & 2) != 0 }

// SetRTR sets the RTR.  nobody uses this anymore.
func (msg *CanMsg) SetRTR() { msg.Flags |= 2 }

// DLC is the Data Length Code which should be 0..8 inclusive.
func (msg *CanMsg) DLC() int { return int(msg.Flags >> 2) }

// Payload is the zero to 8 byte buffer with the CAN message payload.
func (msg *CanMsg) Payload() []byte { return msg.Buf[:msg.DLC()] }

func (msg *CanMsg) String() string {
	if msg.IsExt() {
		return fmt.Sprintf("%03x:% x", msg.Header, msg.Buf[:msg.DLC()])
	}
	return fmt.Sprintf("%08x:% x", msg.Header, msg.Buf[:msg.DLC()])
}

// NewMessage creates a new message with a 10 bit (non extended) address.
func NewMessage(header uint32, payload []byte) *CanMsg {
	r := &CanMsg{Flags: uint8(len(payload) << 2), Header: header & ((1 << 10) - 1)}
	copy(r.Buf[:0], payload)
	return r
}

// NewExtMessage creates a message with a 29 bit address.
func NewExtMessage(header uint32, payload []byte) *CanMsg {
	r := &CanMsg{Flags: uint8(len(payload)<<2) | 1, Header: header & ((1 << 29) - 1)}
	copy(r.Buf[:0], payload)
	return r
}

// SetSpeed writes a message to the device instructing it to set the speed of either or both ports to the given number of kbps.
// Valid speeds are 50 125 250 500 and 1000
func SetSpeed(w io.Writer, port Port, kbps int) error {
	return encode(w, ':', []byte{uint8(port), uint8(kbps >> 8), uint8(kbps)})
}

// Encode writes a CAN message to the device for transmission on either or both ports
func Encode(w io.Writer, port Port, msg *CanMsg) error {
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, port)
	binary.Write(&b, binary.BigEndian, msg)
	if b.Len() != 14 {
		return fmt.Errorf("expected 14 bytes after encoding CanMsg, got %d", b.Len())
	}
	return encode(w, '<', b.Bytes())
}

func encode(w io.Writer, cmdchar byte, b []byte) error {

	var buf bytes.Buffer
	buf.WriteByte(cmdchar)
	buf.WriteString(strings.ToUpper(hex.EncodeToString(b)))
	chk := uint8(0xff)
	for _, v := range buf.Bytes() {
		chk += v
	}
	buf.WriteString(strings.ToUpper(hex.EncodeToString([]byte{chk})))
	buf.WriteString("\r\n")
	_, err := w.Write(buf.Bytes())
	return err
}

// A Decoder reads consecutive messages from the device.
type Decoder struct{ *bufio.Reader }

var (
	ErrNak = errors.New("Set-speed NAK message")
)

// NewDecoder saves the state around a reader to Decode succesive messages.
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
	line = strings.TrimSuffix(line, "\r\n")
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

	var m CanMsg
	err = binary.Read(bytes.NewReader(buf[1:14]), binary.BigEndian, &m)
	return Port(buf[0]), &m, err

}

# emucb201
Go package to receive and transmit CAN messages through the EMUC-B201 W1 card.

Don't use this thing if you can avoid it, but if you're stuck with it, at least here
is some Go code to use it.

The card exposes 2 insulated CAN channels over a usb cdccam device (pseudo serial port)
which shows up as /dev/ttyACMx on Linux and /dev/cu.usbmodemDEMO0000001 on MacOS.

This card comes with pretty terrible firmware which limits the throughput
to about 1000 messages per second, and on top of that the Linux drivers are 
binary-only and not very good, and nothing exists for MacOS, or anything non-C.

Fortunately from some code in the sample application and stracing the demo binary
one is able to reverse engineer the protocol spoken over the pseudo serial device.
Unfortunately for the B202 successor there is a new protocol which is equally undocumented
and harder to reverse engineer.  Just don't use that one at all.

The card sends and receives packets that consist of one of 4 starting characters ':;=<',
followed by a hexadecimal string containing the packet followed by a checksum and a
'\r\n' sequence.

The hex digits must be uppercase.

The 4 types of packets are (see below for legend)
	:ppbbbbcc   				set baudrate         (e.g ":0303E87C")

	;kkcc     					ack/nak set baudrate (either ";019B" or ";009A")
	=ppffhhhhhhhhxx....xxcc     received can packets
	<ppffhhhhhhhhxx....xxcc     send can packets

Where
	pp    : port 01, 02 or 03 for both
	bbbb  : 4 digit hex baudrate in kbps: 50, 125, 250, 500 or 1000 so  0032 007D 00FA 01F4 03E8
	kk    : 00 for bad, 01 for good
	ff    : flags and len
	hhhhhhhh : header   (4 bytes)
	xx...xxx : payload  (8 bytes)
	cc    : checksum

The checksum is computed by adding the ascii representation of the hex string following the start character until
before the cc, and representing the resulting sum /minus 1/ as a hex number again.

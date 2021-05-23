package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/pkg/term"

	emuc ".."
)

var (
	speed = flag.Int("kbps", 0, "set both ports to this speed in kbps")
	debug = flag.Bool("d", false, "log encoded/undecoded strings")
)

func main() {

	flag.Parse()
	if len(flag.Args()) != 1 {
		log.Fatalf("usage: %s /path/to/dev", os.Args[0])
	}

	dev, err := term.Open(flag.Arg(0), term.RawMode, term.Speed(115200)) // speed seems to be ignored
	if err != nil {
		log.Fatal(err)
	}

	var (
		r io.Reader = dev
		w io.Writer = dev
	)

	if *debug {
		r = io.TeeReader(r, os.Stdout)
		w = io.MultiWriter(w, os.Stdout)
	}

	go func() {
		d := emuc.NewDecoder(r)
		for {
			port, msg, err := d.Decode()
			if err != nil {
				log.Fatalln(err)
			}
			if msg == nil {
				fmt.Println("Set Speed ACK")
				continue
			}
			fmt.Println(port, msg)
		}
	}()

	if *speed != 0 {
		log.Printf("Setting speed to %d kbps", *speed)
		emuc.SetSpeed(w, emuc.PORT12, *speed)
	}

	for {
		var (
			port    uint8
			header  uint32
			payload uint64
		)
		_, err := fmt.Scanf("%d %x %x\n", &port, &header, &payload)
		if err != nil {
			log.Fatal(err)
		}

		emuc.Encode(w, emuc.Port(port), emuc.NewExtMessage(header, []byte{uint8(payload >> 56), uint8(payload >> 48), uint8(payload >> 40), uint8(payload >> 32), uint8(payload >> 24), uint8(payload >> 16), uint8(payload >> 8), uint8(payload)}))
	}

}

package boardman

import (
	"log"

	"github.com/spf13/viper"
	"github.com/stianeikeland/go-rpio"
)

var (
//Binary to TD09's gray code.
// BIN0INGRAY = 0
// BIN1INGRAY = 1
// BIN2INGRAY = 3
// BIN3INGRAY = 2
// BIN4INGRAY = 6
// BIN5INGRAY = 7
// BIN6INGRAY = 5
// BIN7INGRAY = 4
// BIN8INGRAY = 12
// BIN9INGRAY = 8
)

func SelectConsole(boardID uint8) {
	gray := viper.GetIntSlice("graymapping")
	ConsolePins := viper.GetIntSlice("uartio")
	log.Print("select console")
	togglePin(ConsolePins, uint8(gray[boardID-1]), uint8(gray[boardID]))
}

func SelectJTAG(boardID uint8) {
	JTAGPins := viper.GetIntSlice("jtagio")
	selectPin(JTAGPins, boardID)
}

func SelectReset(boardID uint8) {
	ResetPins := viper.GetIntSlice("resetio")
	selectPin(ResetPins, boardID)
	selectPin(ResetPins, 0) // release pressed reset
}

func selectPin(pins []int, boardID uint8) {
	// pins []rpio.Pin,
	// err := rpio.Open()
	// if err != nil {
	// 	log.Println("Cannot open GPIO")
	// }
	// defer rpio.Close()
	for _, pin := range pins {
		log.Printf("Set pin%d as output\n", pin)
		rpio.Pin(pin).Output()
	}

	for i, pin := range pins {
		id := (boardID >> uint(3-i) & 1)
		// log.Println("Pin:", pin, "ID:", id)
		if id == 1 {
			rpio.Pin(pin).High()
		} else {
			rpio.Pin(pin).Low()
		}
	}
	// log.Printf("Pin state: %d:%d, %d:%d, %d:%d, %d:%d\n",
	// 	pins[0], pins[0].Read(),
	// 	pins[1], pins[1].Read(),
	// 	pins[2], pins[2].Read(),
	// 	pins[3], pins[3].Read(),
	// )
}

func togglePin(pins []int, prevboardID, boardID uint8) {
	for _, pin := range pins {
		log.Printf("Set pin%d as output\n", pin)
		rpio.Pin(pin).Output()
	}

	if prevboardID == 0 {
		for i, pin := range pins {
			id := (prevboardID >> uint(3-i) & 1)
			if id == 1 {
				rpio.Pin(pin).High()
			} else {
				rpio.Pin(pin).Low()
			}
		}
	}

	masked := prevboardID ^ boardID
	for i, pin := range pins {
		id := (masked >> uint(3-i) & 1)
		// log.Println("Pin:", pin, "ID:", id)
		if id == 1 {
			rpio.Pin(pin).Toggle()
		}
	}
}

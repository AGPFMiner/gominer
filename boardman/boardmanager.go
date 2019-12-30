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

var slot []int
var ConsolePins []int

func SelectConsole(boardID uint8) {
	rpio.Pin(slot[boardID-1]).Toggle()
	log.Printf("board: %d, toggled pin: %d", boardID, slot[boardID-1])
}

func InitConsoleLevel() {
	slot = viper.GetIntSlice("slot")
	ConsolePins = viper.GetIntSlice("uartio")

	for _, pin := range ConsolePins {
		log.Printf("Set pin%d as output\n", pin)
		rpio.Pin(pin).Output()
	}
	rpio.Pin(ConsolePins[0]).Low()  // 6
	rpio.Pin(ConsolePins[1]).Low()  // 5
	rpio.Pin(ConsolePins[2]).Low()  // 4
	rpio.Pin(ConsolePins[3]).High() // 3
	rpio.Pin(ConsolePins[4]).Low()  // 2
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
	gpionums := len(pins)
	for _, pin := range pins {
		log.Printf("Set pin%d as output\n", pin)
		rpio.Pin(pin).Output()
	}

	for i, pin := range pins {
		id := (boardID >> uint(gpionums-1-i) & 1)
		// log.Println("Pin:", pin, "ID:", id)
		if id == 1 {
			rpio.Pin(pin).High()
		} else {
			rpio.Pin(pin).Low()
		}
	}
}

package boardman

import (
	rpio "github.com/stianeikeland/go-rpio"
)

var (
	ConsolePins = [4]rpio.Pin{rpio.Pin(2), rpio.Pin(3), rpio.Pin(4), rpio.Pin(5)}
	JTAGPins    = [4]rpio.Pin{rpio.Pin(6), rpio.Pin(24), rpio.Pin(25), rpio.Pin(26)}
	ResetPins   = [4]rpio.Pin{rpio.Pin(18), rpio.Pin(19), rpio.Pin(12), rpio.Pin(13)}
)

var (
	gray = [10]uint8{0, 1, 3, 2, 6, 7, 5, 4, 12, 8}
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
	selectPin(ConsolePins, gray[boardID])
}

func SelectJTAG(boardID uint8) {
	selectPin(JTAGPins, boardID)
}

func SelectReset(boardID uint8) {
	selectPin(ResetPins, boardID)
	selectPin(ResetPins, 0) // release pressed reset
}

func selectPin(pins [4]rpio.Pin, boardID uint8) {
	// err := rpio.Open()
	// if err != nil {
	// 	log.Println("Cannot open GPIO")
	// }
	// defer rpio.Close()
	for _, pin := range pins {
		pin.Output()
	}

	for i, pin := range pins {
		id := (boardID >> uint(i) & 1)
		// log.Println("Pin:", pin, "ID:", id)
		if id == 1 {
			pin.High()
		} else {
			pin.Low()
		}
	}
	// log.Printf("Pin state: %d:%d, %d:%d, %d:%d, %d:%d\n",
	// 	pins[0], pins[0].Read(),
	// 	pins[1], pins[1].Read(),
	// 	pins[2], pins[2].Read(),
	// 	pins[3], pins[3].Read(),
	// )
}

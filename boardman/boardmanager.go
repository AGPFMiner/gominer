package boardman

import (
	rpio "github.com/stianeikeland/go-rpio"
)

var (
	ConsolePins = [4]rpio.Pin{rpio.Pin(5), rpio.Pin(4), rpio.Pin(3), rpio.Pin(2)}
	JTAGPins    = [4]rpio.Pin{rpio.Pin(26), rpio.Pin(25), rpio.Pin(24), rpio.Pin(6)}
	ResetPins   = [4]rpio.Pin{rpio.Pin(13), rpio.Pin(12), rpio.Pin(19), rpio.Pin(18)}
)

var (
	gray = [13]uint8{0, 3, 2, 6, 7, 5, 4, 12, 13, 15, 14, 10, 11}
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
		id := (boardID >> uint(3-i) & 1)
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

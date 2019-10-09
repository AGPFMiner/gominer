package boardman

import (
	"log"
	"os"
	"strconv"
)

func main() {
	argsWithoutProg := os.Args[1:]
	boardID, _ := strconv.Atoi(argsWithoutProg[1])
	bid := uint8(boardID)
	log.Println(bid)
	switch argsWithoutProg[0] {
	case "console":
		SelectConsole(bid)
	case "jtag":
		SelectJTAG(bid)
	case "reset":
		SelectReset(bid)
	}
}

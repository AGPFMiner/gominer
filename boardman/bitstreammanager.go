package boardman

import (
	"fmt"
	"os/exec"
)

func FlashBitstream(bitstreamPath string) {
	openocdCmd := fmt.Sprintf("'adapter_khz 3000; init; xc7_program xc7.tap; pld load 0 %s; exit'", bitstreamPath)
	cmd := exec.Command("openocd", "-f", "/usr/share/openocd/scripts/interface/raspberrypi-native.cfg", "-f", "/usr/share/openocd/scripts/cpld/xilinx-xc7.cfg", "-c", openocdCmd)
	cmd.Run()
}

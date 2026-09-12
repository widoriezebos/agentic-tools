package hostload

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

func readLoad() (float64, float64, float64, error) {
	raw, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("vm.loadavg: %w", err)
	}
	return parseDarwinLoadavg(raw)
}

// parseDarwinLoadavg decodes the kernel's struct loadavg: three fixed-point
// uint32 averages followed by the scale they are expressed in.
func parseDarwinLoadavg(raw []byte) (float64, float64, float64, error) {
	if len(raw) < 24 {
		return 0, 0, 0, fmt.Errorf("vm.loadavg: %d bytes, want 24", len(raw))
	}
	scale := binary.LittleEndian.Uint64(raw[16:24])
	if scale == 0 {
		return 0, 0, 0, fmt.Errorf("vm.loadavg: zero scale")
	}
	one := float64(binary.LittleEndian.Uint32(raw[0:4])) / float64(scale)
	five := float64(binary.LittleEndian.Uint32(raw[4:8])) / float64(scale)
	fifteen := float64(binary.LittleEndian.Uint32(raw[8:12])) / float64(scale)
	return one, five, fifteen, nil
}

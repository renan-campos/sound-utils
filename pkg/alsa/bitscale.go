package alsa

import "fmt"

func scale8To16(in int) int {
	fmt.Println(in, 257*in)
	return (65535 / 255) * in
	// This method has some distortion.
	var out int
	mask := 1
	for i := 0; i < 8; i++ {
		if (in & mask) > 0 {
			out |= (3 << (2 * i))
		}
		mask <<= 1
	}
	return out
}

func scale32To16(in int) int {
	return in >> 16
}

func scale16To32(in int) int {
	return in << 16
}

func scale8To32(in int) int {
	return in << 24
}

package reader

// v2Alphabet is the custom alphabet used by Among Us V2 game codes.
const v2Alphabet = "QWXRTYLPESDFGHUJKZOCVBINMA"

// IntToGameCode converts an integer lobby code to its string representation.
// Positive values use V1 (4-char ASCII); values <= -1000 use V2 (6-char custom alphabet).
func IntToGameCode(input int32) string {
	if input == 0 {
		return ""
	} else if input <= -1000 {
		return intToGameCodeV2(input)
	} else if input > 0 {
		return intToGameCodeV1(input)
	}
	return ""
}

// intToGameCodeV1 interprets the int32 as 4 raw ASCII bytes (little-endian).
func intToGameCodeV1(input int32) string {
	b := [4]byte{
		byte(input),
		byte(input >> 8),
		byte(input >> 16),
		byte(input >> 24),
	}
	return string(b[:])
}

// intToGameCodeV2 decodes a V2 (6-char) game code from an int32.
func intToGameCodeV2(input int32) string {
	a := int(input) & 0x3ff
	b := (int(input) >> 10) & 0xfffff
	return string([]byte{
		v2Alphabet[a%26],
		v2Alphabet[a/26],
		v2Alphabet[b%26],
		v2Alphabet[(b/26)%26],
		v2Alphabet[(b/676)%26],
		v2Alphabet[(b/17576)%26],
	})
}

// GameCodeToInt converts a lobby code string back to its integer representation.
func GameCodeToInt(code string) int32 {
	if len(code) == 4 {
		return gameCodeToIntV1(code)
	}
	return gameCodeToIntV2(code)
}

func gameCodeToIntV1(code string) int32 {
	if len(code) < 4 {
		return 0
	}
	b := []byte(code)
	return int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16 | int32(b[3])<<24
}

var v2Map = [26]int{25, 21, 19, 10, 8, 11, 12, 13, 22, 15, 16, 6, 24, 23, 18, 7, 0, 3, 9, 4, 14, 20, 1, 2, 5, 17}

func gameCodeToIntV2(code string) int32 {
	if len(code) < 6 {
		return 0
	}
	a := v2Map[code[0]-'A']
	b := v2Map[code[1]-'A']
	c := v2Map[code[2]-'A']
	d := v2Map[code[3]-'A']
	e := v2Map[code[4]-'A']
	f := v2Map[code[5]-'A']
	one := (a + 26*b) & 0x3ff
	two := c + 26*(d+26*(e+26*f))
	result := uint32(one) | ((uint32(two) << 10) & 0x3ffffc00) | 0x80000000
	return int32(result)
}

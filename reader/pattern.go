package reader

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

// patternByte represents one byte in a signature.
// If wildcard is true, the byte value is ignored during matching.
type patternByte struct {
	value    byte
	wildcard bool
}

// parseSignature converts a hex-string signature (e.g. "48 8B ? ? 05 AB") into
// a slice of patternBytes.  "?" or "??" are wildcards.
func parseSignature(sig string) ([]patternByte, error) {
	parts := strings.Fields(sig)
	out := make([]patternByte, 0, len(parts))
	for _, p := range parts {
		if p == "?" || p == "??" {
			out = append(out, patternByte{wildcard: true})
			continue
		}
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid signature byte %q: %w", p, err)
		}
		out = append(out, patternByte{value: byte(v)})
	}
	return out, nil
}

// matchAt returns true if pattern matches buf starting at offset.
func matchAt(buf []byte, offset int, pattern []patternByte) bool {
	if offset+len(pattern) > len(buf) {
		return false
	}
	for i, pb := range pattern {
		if pb.wildcard {
			continue
		}
		if buf[offset+i] != pb.value {
			return false
		}
	}
	return true
}

// scanChunk scans a byte slice for the pattern, returning the offset of the
// skip-th match (-1 if not found).
func scanChunk(buf []byte, pattern []patternByte, skip int) int {
	found := 0
	for i := 0; i <= len(buf)-len(pattern); i++ {
		if matchAt(buf, i, pattern) {
			if found == skip {
				return i
			}
			found++
		}
	}
	return -1
}

// findPattern scans GameAssembly.dll for a byte-pattern signature and returns
// the resolved offset according to the same logic as the original TypeScript
// implementation.
//
// Parameters:
//
//	sig           — hex signature string with optional "?" wildcards
//	patternOffset — byte offset within the matched instruction to read a value
//	addressOffset — added to the resolved result
//	relative      — if true, treat the read value as a relative offset
//	getLocation   — if true, return the instruction location rather than
//	                dereferencing the value stored there
//	skip          — index of the match to use (0 = first)
//
// Returns the resolved address as an offset relative to modBaseAddr.
func (gr *GameReader) findPattern(
	sig string,
	patternOffset int,
	addressOffset int,
	relative bool,
	getLocation bool,
	skip int,
) (uintptr, error) {
	pattern, err := parseSignature(sig)
	if err != nil {
		return 0, fmt.Errorf("findPattern: %w", err)
	}

	// Read GameAssembly.dll in chunks to avoid one huge allocation.
	const chunkSize = 4 * 1024 * 1024 // 4 MiB
	const overlap = 256               // keep overlap to not miss matches at chunk boundaries

	moduleSize := int(gr.moduleSize)
	base := gr.moduleBase

	found := -1 // absolute offset within the module
	globalSkip := skip

	for offset := 0; offset < moduleSize; offset += chunkSize - overlap {
		readSize := chunkSize
		if offset+readSize > moduleSize {
			readSize = moduleSize - offset
		}
		if readSize < len(pattern) {
			break
		}

		buf, err := gr.proc.readBytes(base+uintptr(offset), readSize)
		if err != nil {
			continue
		}

		idx := scanChunk(buf, pattern, globalSkip)
		if idx >= 0 {
			found = offset + idx
			break
		}
		// Count how many matches were in this chunk so we adjust skip
		// for the next chunk.
		for i := 0; i <= len(buf)-len(pattern); i++ {
			if matchAt(buf, i, pattern) {
				globalSkip--
			}
		}
	}

	if found < 0 {
		return 0, fmt.Errorf("pattern %q not found (skip=%d)", sig, skip)
	}

	instructionLocation := uintptr(found) + uintptr(patternOffset)

	if getLocation {
		return instructionLocation + uintptr(addressOffset), nil
	}

	// Read the 4-byte value encoded at instructionLocation inside the module.
	valueBuf, err := gr.proc.readBytes(base+instructionLocation, 4)
	if err != nil {
		return 0, fmt.Errorf("findPattern read value at 0x%x: %w", base+instructionLocation, err)
	}
	offsetAddr := uintptr(binary.LittleEndian.Uint32(valueBuf))

	if gr.proc.is64bit || relative {
		// 64-bit RIP-relative or explicit relative: result = value + instructionLocation + addressOffset
		return offsetAddr + instructionLocation + uintptr(addressOffset), nil
	}
	// 32-bit absolute: result = value - modBaseAddr
	return offsetAddr - base, nil
}

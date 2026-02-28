package reader

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/NhProGamer/AmongUsReader/types"
)

// structResult holds the decoded fields from a raw player buffer.
// Fields are stored by name using their natural Go type.
type structResult map[string]interface{}

// parseStruct decodes a raw byte buffer according to the list of StructMembers
// defined in the offsets JSON (equivalent to structron in the TypeScript code).
//
// Supported types mirror the structron types used by BetterCrewLink:
//
//	INT / INT_BE    → int32
//	UINT / UINT_BE  → uint32
//	SHORT / SHORT_BE → int16
//	USHORT / USHORT_BE → uint16
//	FLOAT           → float32
//	CHAR            → byte (uint8)
//	BYTE            → uint8
//	SKIP(n)         → advance cursor by n bytes without storing a value
func parseStruct(buf []byte, members []types.StructMember) (structResult, error) {
	result := make(structResult, len(members))
	cursor := 0

	for _, m := range members {
		switch m.Type {
		case types.StructTypeSKIP:
			skip := 0
			if m.Skip != nil {
				skip = *m.Skip
			}
			cursor += skip

		case types.StructTypeINT:
			if cursor+4 > len(buf) {
				return result, fmt.Errorf("parseStruct: INT out of bounds at %d (member %q)", cursor, m.Name)
			}
			result[m.Name] = int32(binary.LittleEndian.Uint32(buf[cursor:]))
			cursor += 4

		case types.StructTypeINTBE:
			if cursor+4 > len(buf) {
				return result, fmt.Errorf("parseStruct: INT_BE out of bounds at %d (member %q)", cursor, m.Name)
			}
			result[m.Name] = int32(binary.BigEndian.Uint32(buf[cursor:]))
			cursor += 4

		case types.StructTypeUINT:
			if cursor+4 > len(buf) {
				return result, fmt.Errorf("parseStruct: UINT out of bounds at %d (member %q)", cursor, m.Name)
			}
			result[m.Name] = binary.LittleEndian.Uint32(buf[cursor:])
			cursor += 4

		case types.StructTypeUINTBE:
			if cursor+4 > len(buf) {
				return result, fmt.Errorf("parseStruct: UINT_BE out of bounds at %d (member %q)", cursor, m.Name)
			}
			result[m.Name] = binary.BigEndian.Uint32(buf[cursor:])
			cursor += 4

		case types.StructTypeSHORT:
			if cursor+2 > len(buf) {
				return result, fmt.Errorf("parseStruct: SHORT out of bounds at %d (member %q)", cursor, m.Name)
			}
			result[m.Name] = int16(binary.LittleEndian.Uint16(buf[cursor:]))
			cursor += 2

		case types.StructTypeSHORTBE:
			if cursor+2 > len(buf) {
				return result, fmt.Errorf("parseStruct: SHORT_BE out of bounds at %d (member %q)", cursor, m.Name)
			}
			result[m.Name] = int16(binary.BigEndian.Uint16(buf[cursor:]))
			cursor += 2

		case types.StructTypeUSHORT:
			if cursor+2 > len(buf) {
				return result, fmt.Errorf("parseStruct: USHORT out of bounds at %d (member %q)", cursor, m.Name)
			}
			result[m.Name] = binary.LittleEndian.Uint16(buf[cursor:])
			cursor += 2

		case types.StructTypeUSHORTBE:
			if cursor+2 > len(buf) {
				return result, fmt.Errorf("parseStruct: USHORT_BE out of bounds at %d (member %q)", cursor, m.Name)
			}
			result[m.Name] = binary.BigEndian.Uint16(buf[cursor:])
			cursor += 2

		case types.StructTypeFLOAT:
			if cursor+4 > len(buf) {
				return result, fmt.Errorf("parseStruct: FLOAT out of bounds at %d (member %q)", cursor, m.Name)
			}
			bits := binary.LittleEndian.Uint32(buf[cursor:])
			result[m.Name] = math.Float32frombits(bits)
			cursor += 4

		case types.StructTypeCHAR, types.StructTypeBYTE:
			if cursor+1 > len(buf) {
				return result, fmt.Errorf("parseStruct: BYTE out of bounds at %d (member %q)", cursor, m.Name)
			}
			result[m.Name] = buf[cursor]
			cursor++

		default:
			return result, fmt.Errorf("parseStruct: unknown member type %q (member %q)", m.Type, m.Name)
		}
	}
	return result, nil
}

// offsetOf returns the byte offset of the named member in the struct layout
// (analogous to Struct.getOffsetByName in structron).
func offsetOf(members []types.StructMember, name string) (int, bool) {
	cursor := 0
	for _, m := range members {
		if m.Type == types.StructTypeSKIP {
			skip := 0
			if m.Skip != nil {
				skip = *m.Skip
			}
			cursor += skip
			continue
		}
		if m.Name == name {
			return cursor, true
		}
		cursor += memberSize(m.Type)
	}
	return 0, false
}

// memberSize returns the byte size for a given StructMemberType.
func memberSize(t types.StructMemberType) int {
	switch t {
	case types.StructTypeINT, types.StructTypeINTBE,
		types.StructTypeUINT, types.StructTypeUINTBE,
		types.StructTypeFLOAT:
		return 4
	case types.StructTypeSHORT, types.StructTypeSHORTBE,
		types.StructTypeUSHORT, types.StructTypeUSHORTBE:
		return 2
	case types.StructTypeCHAR, types.StructTypeBYTE:
		return 1
	default:
		return 0
	}
}

// srGetUint32 returns a uint32 field from a structResult, or 0 if absent/wrong type.
func srGetUint32(r structResult, key string) uint32 {
	v, ok := r[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case uint32:
		return val
	case int32:
		return uint32(val)
	case uint8:
		return uint32(val)
	case uint16:
		return uint32(val)
	case int16:
		return uint32(val)
	}
	return 0
}

// srGetInt32 returns an int32 field from a structResult, or 0 if absent.
func srGetInt32(r structResult, key string) int32 {
	v, ok := r[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case int32:
		return val
	case uint32:
		return int32(val)
	case int16:
		return int32(val)
	case uint8:
		return int32(val)
	}
	return 0
}

// srHas returns true if the structResult contains the named key.
func srHas(r structResult, key string) bool {
	_, ok := r[key]
	return ok
}

package reader

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"

	"github.com/NhProGamer/AmongUsReader/types"
)

const rainbowColorID = 50 // RainbowColorId from the original TS

// parsePlayer decodes a player from a raw memory buffer + live pointer reads.
// ptr is the absolute address of the PlayerData struct; buf is its raw bytes;
// localClientID is the InnerNetClient.clientId used to identify the local player.
func (gr *GameReader) parsePlayer(ptr uintptr, buf []byte, localClientID uint32) *types.Player {
	offsets := gr.offsets
	if offsets == nil {
		return nil
	}

	data, err := parseStruct(buf, offsets.Player.Struct)
	if err != nil {
		return nil
	}

	// In 64-bit builds the struct fields that hold pointers are stored as
	// 32-bit values (truncated), so we must re-read them as full pointer-sized
	// values from process memory.
	var objectPtr, outfitsPtr, taskPtr, rolePtr uintptr

	if gr.proc.is64bit {
		if off, ok := offsetOf(offsets.Player.Struct, "objectPtr"); ok {
			objectPtr, _ = gr.readPointerDirect(ptr + uintptr(off))
		}
		if off, ok := offsetOf(offsets.Player.Struct, "outfitsPtr"); ok {
			outfitsPtr, _ = gr.readPointerDirect(ptr + uintptr(off))
		}
		if off, ok := offsetOf(offsets.Player.Struct, "taskPtr"); ok {
			taskPtr, _ = gr.readPointerDirect(ptr + uintptr(off))
		}
		if off, ok := offsetOf(offsets.Player.Struct, "rolePtr"); ok {
			rolePtr, _ = gr.readPointerDirect(ptr + uintptr(off))
		}
	} else {
		objectPtr = uintptr(srGetUint32(data, "objectPtr"))
		outfitsPtr = uintptr(srGetUint32(data, "outfitsPtr"))
		taskPtr = uintptr(srGetUint32(data, "taskPtr"))
		rolePtr = uintptr(srGetUint32(data, "rolePtr"))
	}

	disconnected := srGetUint32(data, "disconnected")
	colorID := srGetUint32(data, "color")
	id := srGetUint32(data, "id")
	impostor := srGetUint32(data, "impostor")
	dead := srGetUint32(data, "dead")

	clientID, _ := gr.readUint32(objectPtr, offsets.Player.ClientID)
	isLocal := clientID == localClientID && disconnected == 0

	posOffX := offsets.Player.RemoteX
	posOffY := offsets.Player.RemoteY
	if isLocal {
		posOffX = offsets.Player.LocalX
		posOffY = offsets.Player.LocalY
	}

	x := gr.readFloat(objectPtr, posOffX)
	y := gr.readFloat(objectPtr, posOffY)
	currentOutfit, _ := gr.readUint32(objectPtr, offsets.Player.CurrentOutfit)
	isDummy, _ := gr.readUint32(objectPtr, offsets.Player.IsDummy)

	var name string
	var shiftedColor int32 = -1
	var hatID, skinID, visorID, petID string

	if srHas(data, "name") {
		nameAddr := uintptr(srGetUint32(data, "name"))
		name = stripTags(gr.readString(nameAddr, 1000))
	} else {
		// New-style: outfits dictionary
		gr.readDictionary(outfitsPtr, 6, func(kPtr, vPtr uintptr, i int) {
			key, _ := gr.readInt32Direct(kPtr)
			val, _ := gr.readPointerDirect(vPtr)
			if key == 0 && i == 0 {
				// readAddr dereferences the last offset (matches TS readMemory('pointer', val, offsets))
				namePtr, _ := gr.readAddr(val, offsets.Player.Outfit.PlayerName)
				colorID = func() uint32 {
					v, _ := gr.readUint32(val, offsets.Player.Outfit.ColorID)
					return v
				}()
				name = stripTags(gr.readString(namePtr, 1000))
				hatPtr, _ := gr.readAddr(val, offsets.Player.Outfit.HatID)
				hatID = gr.readString(hatPtr, 100)
				skinPtr, _ := gr.readAddr(val, offsets.Player.Outfit.SkinID)
				skinID = gr.readString(skinPtr, 100)
				visorPtr, _ := gr.readAddr(val, offsets.Player.Outfit.VisorID)
				visorID = gr.readString(visorPtr, 100)
				if currentOutfit != 0 && currentOutfit <= 10 {
					// not returning — continue to check for shiftedColor
				}
			} else if key == int32(currentOutfit) {
				sc, _ := gr.readUint32(val, offsets.Player.Outfit.ColorID)
				shiftedColor = int32(sc)
			}
		})

		// roleTeam determines impostor
		roleTeam, _ := gr.readUint32(rolePtr, offsets.Player.RoleTeam)
		impostor = roleTeam
	}

	name = stripTags(name)

	bugged := false
	if disconnected != 0 || int(colorID) < 0 || int(colorID) >= len(gr.playerColors) {
		x = 9999
		y = 9999
		bugged = true
	}

	// Round to 4 decimal places (mirror TS toFixed(4))
	x = roundFloat32(x, 4)
	y = roundFloat32(y, 4)

	nameHash := hashCode(name)

	effectiveColorID := colorID
	if gr.rainbowColor >= 0 && int(colorID) == gr.rainbowColor {
		effectiveColorID = rainbowColorID
	}

	inVent, _ := gr.readUint32(objectPtr, offsets.Player.InVent)

	return &types.Player{
		Ptr:          ptr,
		ID:           id,
		ClientID:     clientID,
		Name:         name,
		NameHash:     nameHash,
		ColorID:      effectiveColorID,
		ShiftedColor: shiftedColor,
		HatID:        hatID,
		PetID:        petID,
		SkinID:       skinID,
		VisorID:      visorID,
		Disconnected: disconnected != 0,
		IsImpostor:   impostor == 1,
		IsDead:       dead == 1,
		InVent:       inVent > 0,
		IsLocal:      isLocal,
		IsDummy:      isDummy != 0,
		Bugged:       bugged,
		X:            x,
		Y:            y,
		ObjectPtr:    objectPtr,
		TaskPtr:      taskPtr,
	}
}

// readString reads a Unity IL2CPP string object from the given address.
// The layout is:
//
//	+0x08 (32-bit) / +0x10 (64-bit) : int32 length (in UTF-16 chars)
//	+0x0C (32-bit) / +0x14 (64-bit) : UTF-16LE character data (length*2 bytes)
func (gr *GameReader) readString(address uintptr, maxLength int) string {
	if address == 0 || gr.proc == nil {
		return ""
	}

	lenOff := uintptr(0x08)
	dataOff := uintptr(0x0c)
	if gr.proc.is64bit {
		lenOff = 0x10
		dataOff = 0x14
	}

	lenBuf, err := gr.proc.readBytes(address+lenOff, 4)
	if err != nil {
		return ""
	}
	length := int(binary.LittleEndian.Uint32(lenBuf))
	if length <= 0 {
		return ""
	}
	if length > maxLength {
		length = maxLength
	}

	raw, err := gr.proc.readBytes(address+dataOff, length*2)
	if err != nil {
		return ""
	}

	// Decode UTF-16LE
	u16 := make([]uint16, length)
	for i := 0; i < length; i++ {
		u16[i] = binary.LittleEndian.Uint16(raw[i*2:])
	}
	decoded := string(utf16.Decode(u16))
	// Remove null chars
	return strings.ReplaceAll(decoded, "\x00", "")
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// stripTags removes HTML/XML-like tags (e.g. <color=...>) from a string.
func stripTags(s string) string {
	var out []byte
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				out = append(out, s[i])
			}
		}
	}
	return string(out)
}

// hashCode computes a Java-style hash of a string (mirrors the TS hashCode method).
func hashCode(s string) int32 {
	var h int32
	for _, c := range s {
		h = h*31 + int32(c)
	}
	return h
}

// roundFloat32 rounds a float32 to n decimal places.
func roundFloat32(v float32, n int) float32 {
	pow := float32(1)
	for i := 0; i < n; i++ {
		pow *= 10
	}
	return float32(int32(v*pow+0.5)) / pow
}

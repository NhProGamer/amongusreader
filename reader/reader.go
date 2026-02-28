package reader

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NhProGamer/AmongUsReader/types"
)

// tickRate controls how many times per second the reader polls game memory.
const tickRate = 5

// GameReader holds all runtime state for the Among Us memory reader.
type GameReader struct {
	proc          *processHandle
	moduleBase    uintptr
	moduleSize    uint32
	offsets       *types.IOffsets
	loadedMod     types.Mod
	gamePath      string
	gameCode      string
	currentServer string
	oldGameState  types.GameState
	lastState     types.AmongUsState
	lastPlayerPtr uintptr

	menuUpdateTimer   int
	checkProcDelay    int
	isLocalGame       bool
	colorsInitialized bool
	rainbowColor      int
	playerColors      [][2]string // [main, shadow] hex colors
	oldMeetingHud     bool
}

// Start launches the reader loop in a goroutine and returns a read-only channel
// that receives a new AmongUsState on every tick when the game is running.
// The channel is closed when the reader encounters a fatal error.
func Start() <-chan types.AmongUsState {
	ch := make(chan types.AmongUsState, 4)
	go func() {
		defer close(ch)
		gr := &GameReader{
			menuUpdateTimer: 20,
			rainbowColor:    -9999,
			oldGameState:    types.GameStateUnknown,
			loadedMod:       types.ModList[0],
		}
		ticker := time.NewTicker(time.Second / tickRate)
		defer ticker.Stop()
		for range ticker.C {
			state, err := gr.tick()
			if err != nil {
				log.Printf("[reader] tick error: %v", err)
			}
			if state != nil {
				ch <- *state
			}
		}
	}()
	return ch
}

// tick is called at tickRate Hz. It manages process detection, initialises
// offsets when a new process is found, reads game state and returns a snapshot.
func (gr *GameReader) tick() (*types.AmongUsState, error) {
	// Re-check process every 30 ticks (~6 s at 5 Hz).
	gr.checkProcDelay--
	if gr.checkProcDelay <= 0 {
		gr.checkProcDelay = 30
		if err := gr.checkProcessOpen(); err != nil {
			gr.checkProcDelay = 50 // ~10 s backoff on attach error
			return nil, err
		}
	}

	if gr.proc == nil || gr.offsets == nil {
		return nil, nil
	}

	return gr.readGameState()
}

// checkProcessOpen finds and (re)opens Among Us, initialising offsets on first
// attach or after the process restarts.
func (gr *GameReader) checkProcessOpen() error {
	proc, err := openAmongUs()
	if err != nil {
		// Game not running — reset state if we had one.
		if gr.proc != nil {
			gr.proc.Close()
			gr.proc = nil
			gr.offsets = nil
			gr.colorsInitialized = false
			log.Println("[reader] Among Us closed")
		}
		return nil // not a fatal error
	}

	// Same PID → nothing to do.
	if gr.proc != nil && proc.pid == gr.proc.pid {
		proc.Close()
		return nil
	}

	// New or restarted process.
	if gr.proc != nil {
		gr.proc.Close()
	}
	gr.proc = proc

	base, size, err := gr.proc.findModule("GameAssembly.dll")
	if err != nil {
		gr.proc.Close()
		gr.proc = nil
		return fmt.Errorf("findModule: %w", err)
	}
	gr.moduleBase = base
	gr.moduleSize = size
	gr.proc.is64bit = gr.proc.redetectArchWithBase(base)

	path, err := gr.proc.getProcessPath()
	if err == nil {
		gr.gamePath = path
		gr.loadedMod = getInstalledMod(path)
	}

	gr.colorsInitialized = false
	gr.rainbowColor = -9999
	gr.currentServer = ""
	gr.gameCode = ""
	gr.lastPlayerPtr = 0
	gr.menuUpdateTimer = 20
	gr.oldGameState = types.GameStateUnknown

	log.Printf("[reader] Attached to Among Us (pid=%d, 64bit=%v, modBase=0x%x, modSize=%d, mod=%s)",
		proc.pid, proc.is64bit, base, size, gr.loadedMod.ID)

	if err := gr.initializeOffsets(); err != nil {
		gr.proc.Close()
		gr.proc = nil
		return fmt.Errorf("initializeOffsets: %w", err)
	}
	return nil
}

// initializeOffsets fetches the offset lookup, scans for the broadcastVersion,
// downloads the correct offsets file, and resolves all pattern-scan addresses.
func (gr *GameReader) initializeOffsets() error {
	lookup, err := FetchOffsetLookup()
	if err != nil {
		return fmt.Errorf("FetchOffsetLookup: %w", err)
	}

	// Determine broadcastVersion via pattern scan.
	var broadcastSig types.ISignature
	if gr.proc.is64bit {
		broadcastSig = lookup.Patterns.X64.BroadcastVersion
	} else {
		broadcastSig = lookup.Patterns.X86.BroadcastVersion
	}

	broadcastAddr, err := gr.findPattern(
		broadcastSig.Sig,
		broadcastSig.PatternOffset,
		broadcastSig.AddressOffset,
		false, true, 0,
	)
	if err != nil {
		return fmt.Errorf("broadcastVersion pattern: %w", err)
	}

	broadcastVersion, err := gr.readInt32(gr.moduleBase, []int{int(broadcastAddr)})
	if err != nil {
		return fmt.Errorf("read broadcastVersion: %w", err)
	}
	log.Printf("[reader] broadcastVersion = %d", broadcastVersion)

	versionKey := fmt.Sprintf("%d", broadcastVersion)
	entry, ok := lookup.Versions[versionKey]
	if !ok {
		entry = lookup.Versions["default"]
	}

	offsets, err := FetchOffsets(gr.proc.is64bit, entry.File, entry.OffsetsVersion)
	if err != nil {
		return fmt.Errorf("FetchOffsets: %w", err)
	}
	gr.offsets = offsets
	gr.oldMeetingHud = offsets.OldMeetingHud

	// Resolve all pattern-scan signatures into their concrete offsets.
	type patchTarget struct {
		sig    types.ISignature
		dest   *[]int
		rel    bool
		getLoc bool
	}

	innerNetClient, err := gr.resolveSignature(offsets.Signatures.InnerNetClient)
	if err != nil {
		return fmt.Errorf("sig innerNetClient: %w", err)
	}
	meetingHud, err := gr.resolveSignature(offsets.Signatures.MeetingHud)
	if err != nil {
		return fmt.Errorf("sig meetingHud: %w", err)
	}
	gameData, err := gr.resolveSignature(offsets.Signatures.GameData)
	if err != nil {
		return fmt.Errorf("sig gameData: %w", err)
	}
	shipStatus, err := gr.resolveSignature(offsets.Signatures.ShipStatus)
	if err != nil {
		return fmt.Errorf("sig shipStatus: %w", err)
	}
	miniGame, err := gr.resolveSignature(offsets.Signatures.MiniGame)
	if err != nil {
		return fmt.Errorf("sig miniGame: %w", err)
	}
	palette, err := gr.resolveSignature(offsets.Signatures.Palette)
	if err != nil {
		return fmt.Errorf("sig palette: %w", err)
	}
	playerControl, err := gr.resolveSignature(offsets.Signatures.PlayerControl)
	if err != nil {
		return fmt.Errorf("sig playerControl: %w", err)
	}
	serverManager, err := gr.resolveSignature(offsets.Signatures.ServerManager)
	if err != nil {
		return fmt.Errorf("sig serverManager: %w", err)
	}

	if offsets.NewGameOptions {
		gameOptionsManager, err := gr.resolveSignature(offsets.Signatures.GameOptionsManager)
		if err != nil {
			return fmt.Errorf("sig gameOptionsManager: %w", err)
		}
		offsets.GameOptionsData[0] = int(gameOptionsManager)
	} else {
		offsets.GameOptionsData[0] = int(playerControl)
	}

	offsets.Palette[0] = int(palette)
	offsets.MeetingHud[0] = int(meetingHud)
	offsets.AllPlayersPtr[0] = int(gameData)
	offsets.InnerNetClient.Base[0] = int(innerNetClient)
	offsets.ShipStatus[0] = int(shipStatus)
	offsets.MiniGame[0] = int(miniGame)
	offsets.ServerManagerCurrentServer[0] = int(serverManager)

	log.Printf("[reader] Offsets initialized (serverManager=0x%x)", serverManager)
	return nil
}

// resolveSignature calls findPattern with the standard non-relative, non-getLocation
// parameters used for most signatures.
func (gr *GameReader) resolveSignature(sig types.ISignature) (uintptr, error) {
	return gr.findPattern(sig.Sig, sig.PatternOffset, sig.AddressOffset, false, false, 0)
}

// readGameState reads the full game state from memory and returns it as a snapshot.
func (gr *GameReader) readGameState() (*types.AmongUsState, error) {
	offsets := gr.offsets

	gr.loadColors()

	// ── Game state ──────────────────────────────────────────────────────────
	meetingHud, _ := gr.readAddr(gr.moduleBase, offsets.MeetingHud)
	var meetingHudState uint32 = 4
	if meetingHud != 0 {
		cachePtr, _ := gr.readAddr(meetingHud, offsets.ObjectCachePtr)
		if cachePtr != 0 {
			v, _ := gr.readUint32(meetingHud, offsets.MeetingHudState)
			meetingHudState = v
		}
	}

	innerNetClient, _ := gr.readAddr(gr.moduleBase, offsets.InnerNetClient.Base)
	rawGameState, _ := gr.readUint32Offset(innerNetClient, offsets.InnerNetClient.GameState)

	var state types.GameState
	switch rawGameState {
	case 0:
		state = types.GameStateMenu
	case 1, 3:
		state = types.GameStateLobby
	default:
		if meetingHudState < 4 {
			state = types.GameStateDiscussion
		} else {
			state = types.GameStateTasks
		}
	}

	// ── Lobby code ──────────────────────────────────────────────────────────
	var lobbyCodeInt int32
	if state != types.GameStateMenu {
		v, _ := gr.readInt32Offset(innerNetClient, offsets.InnerNetClient.GameID)
		lobbyCodeInt = v
	} else {
		lobbyCodeInt = -1
	}

	if state == types.GameStateMenu {
		gr.gameCode = ""
	} else if lobbyCodeInt != gr.lastState.LobbyCodeInt {
		gr.gameCode = IntToGameCode(lobbyCodeInt)
	}

	// ── Players ─────────────────────────────────────────────────────────────
	allPlayersPtr, _ := gr.readAddr(gr.moduleBase, offsets.AllPlayersPtr)
	allPlayers, _ := gr.readAddr(allPlayersPtr, offsets.AllPlayers)
	playerCount, _ := gr.readUint32(allPlayersPtr, offsets.PlayerCount)
	hostID, _ := gr.readUint32Offset(innerNetClient, offsets.InnerNetClient.HostID)
	clientID, _ := gr.readUint32Offset(innerNetClient, offsets.InnerNetClient.ClientID)

	gr.isLocalGame = lobbyCodeInt == 32

	// Update current server when transitioning out of menu.
	if gr.currentServer == "" ||
		(gr.oldGameState != state &&
			(gr.oldGameState == types.GameStateMenu || gr.oldGameState == types.GameStateUnknown)) {
		gr.readCurrentServer()
	}

	var players []types.Player
	var localPlayer *types.Player
	var lightRadius float32 = 1.0
	var comsSabotaged bool
	var currentCamera types.CameraLocation = types.CameraNone
	var mapType types.MapType
	var maxPlayers = 10
	var closedDoors []int

	ptrSize := uintptr(4)
	if gr.proc.is64bit {
		ptrSize = 8
	}

	if (gr.gameCode != "" || gr.isLocalGame) && playerCount > 0 {
		playerAddrPtr := allPlayers + uintptr(offsets.PlayerAddrPtr)
		count := int(playerCount)
		if count > 40 {
			count = 40
		}
		for i := 0; i < count; i++ {
			addr, last, err := gr.offsetAddress(playerAddrPtr, offsets.Player.Offsets)
			if err != nil || addr == 0 {
				playerAddrPtr += ptrSize
				continue
			}
			rawPtr := addr + uintptr(last)
			playerBuf, err := gr.proc.readBytes(rawPtr, offsets.Player.BufferLength)
			if err != nil {
				playerAddrPtr += ptrSize
				continue
			}
			p := gr.parsePlayer(rawPtr, playerBuf, clientID)
			playerAddrPtr += ptrSize
			if p == nil || state == types.GameStateMenu {
				continue
			}
			if gr.isLocalGame && p.ClientID == hostID {
				gr.gameCode = fmt.Sprintf("%d", int32(p.NameHash)%99999)
			}
			if p.IsLocal {
				localPlayer = p
			}
			players = append(players, *p)
		}

		if localPlayer != nil {
			lightRadius = gr.readFloat(localPlayer.ObjectPtr, offsets.LightRadius)
		}

		gameOptionsPtr, _ := gr.readAddr(gr.moduleBase, offsets.GameOptionsData)
		maxPlayers = int(gr.readByte(gameOptionsPtr, offsets.GameOptionsMaxPlayers))
		mapType = types.MapType(gr.readByte(gameOptionsPtr, offsets.GameOptionsMapID))

		if state == types.GameStateTasks {
			shipPtr, _ := gr.readAddr(gr.moduleBase, offsets.ShipStatus)
			systemsPtr, _ := gr.readAddr(shipPtr, offsets.ShipStatusSystems)

			if systemsPtr != 0 {
				gr.readDictionary(systemsPtr, 47, func(kPtr, vPtr uintptr, _ int) {
					key, _ := gr.readInt32Direct(kPtr)
					if key == 14 {
						value, _ := gr.readPointerDirect(vPtr)
						switch mapType {
						case types.MapTypeAirship, types.MapTypePolus,
							types.MapTypeFungle, types.MapTypeTheSkeld, 105: // Submerged
							v, _ := gr.readUint32(value, offsets.HudOverrideIsActive)
							comsSabotaged = v == 1
						case types.MapTypeMiraHQ:
							v, _ := gr.readUint32(value, offsets.HqHudCompletedConsoles)
							comsSabotaged = v < 2
						}
					} else if key == 18 && mapType == types.MapTypeMiraHQ {
						value, _ := gr.readPointerDirect(vPtr)
						lower, _ := gr.readUint32(value, offsets.DeconDoorLowerOpen)
						upper, _ := gr.readUint32(value, offsets.DeconDoorUpperOpen)
						if lower == 0 {
							closedDoors = append(closedDoors, 0)
						}
						if upper == 0 {
							closedDoors = append(closedDoors, 1)
						}
					}
				})
			}

			// Camera detection
			miniGamePtr, _ := gr.readAddr(gr.moduleBase, offsets.MiniGame)
			miniGameCache, _ := gr.readAddr(miniGamePtr, offsets.ObjectCachePtr)
			if miniGameCache != 0 && localPlayer != nil {
				switch mapType {
				case types.MapTypePolus, types.MapTypeAirship:
					camID, _ := gr.readUint32(miniGamePtr, offsets.PlanetSurveillanceCurrentCamera)
					camCount, _ := gr.readUint32(miniGamePtr, offsets.PlanetSurveillanceCamarasCount)
					if camID >= 0 && camID <= 5 && camCount == 6 {
						currentCamera = types.CameraLocation(camID)
					}
				case types.MapTypeTheSkeld:
					roomCount, _ := gr.readUint32(miniGamePtr, offsets.SurveillanceFilteredRoomsCount)
					if roomCount == 4 {
						dx := localPlayer.X - (-12.9364)
						dy := localPlayer.Y - (-2.7928)
						dist := float32(dx*dx + dy*dy) // compare squared to avoid sqrt
						if dist < 0.36 {               // 0.6²
							currentCamera = types.CameraSkeld
						}
					}
				}
			}

			// Door state (non Mira HQ maps)
			if mapType != types.MapTypeMiraHQ {
				shipPtr2, _ := gr.readAddr(gr.moduleBase, offsets.ShipStatus)
				allDoors, _ := gr.readAddr(shipPtr2, offsets.ShipStatusAllDoors)
				doorCount, _ := gr.readUint32(allDoors, offsets.PlayerCount)
				if doorCount > 16 {
					doorCount = 16
				}
				for d := uint32(0); d < doorCount; d++ {
					doorOff := uintptr(offsets.PlayerAddrPtr) + uintptr(d)*ptrSize
					doorPtr, err := gr.readPointerDirect(allDoors + doorOff)
					if err != nil {
						continue
					}
					isOpen, _ := gr.readInt32Offset(doorPtr, offsets.DoorIsOpen)
					if isOpen != 1 {
						closedDoors = append(closedDoors, int(d))
					}
				}
			}
		}
	}

	// Menu transition guard (mirrors TS logic).
	if gr.oldGameState == types.GameStateMenu &&
		state == types.GameStateLobby &&
		gr.menuUpdateTimer > 0 &&
		(gr.lastPlayerPtr == allPlayers || !hasLocal(players)) {
		state = types.GameStateMenu
		gr.menuUpdateTimer--
	} else {
		gr.menuUpdateTimer = 20
		gr.lastPlayerPtr = allPlayers
	}

	lobbyCode := gr.gameCode
	if lobbyCode == "" || state == types.GameStateMenu {
		lobbyCode = "MENU"
	}

	newState := types.AmongUsState{
		LobbyCode:    lobbyCode,
		LobbyCodeInt: lobbyCodeInt,
		Players:      players,
		GameState: func() types.GameState {
			if lobbyCode == "MENU" {
				return types.GameStateMenu
			}
			return state
		}(),
		OldGameState:       gr.oldGameState,
		IsHost:             hostID != 0 && clientID != 0 && hostID == clientID,
		HostID:             hostID,
		ClientID:           clientID,
		ComsSabotaged:      comsSabotaged,
		CurrentCamera:      currentCamera,
		LightRadius:        lightRadius,
		LightRadiusChanged: lightRadius != gr.lastState.LightRadius,
		Map:                mapType,
		Mod:                gr.loadedMod.ID,
		ClosedDoors:        closedDoors,
		CurrentServer:      gr.currentServer,
		MaxPlayers:         maxPlayers,
		OldMeetingHud:      gr.oldMeetingHud,
	}
	if closedDoors == nil {
		newState.ClosedDoors = []int{}
	}

	gr.lastState = newState
	gr.oldGameState = state

	return &newState, nil
}

// loadColors reads the player color palette from game memory (once per process attach).
func (gr *GameReader) loadColors() {
	if gr.colorsInitialized {
		return
	}
	offsets := gr.offsets
	palettePtr, err := gr.readAddr(gr.moduleBase, offsets.Palette)
	if err != nil {
		return
	}
	playerColorsPtr, err := gr.readAddr(palettePtr, offsets.PalettePlayerColor)
	if err != nil {
		return
	}
	shadowColorsPtr, err := gr.readAddr(palettePtr, offsets.PaletteShadowColor)
	if err != nil {
		return
	}
	colorLen, err := gr.readUint32(shadowColorsPtr, offsets.PlayerCount)
	if err != nil || colorLen <= 0 || colorLen > 300 {
		return
	}
	if gr.loadedMod.ID == types.ModTheOtherRoles && colorLen <= 18 {
		return
	}

	gr.rainbowColor = -9999
	colors := make([][2]string, colorLen)
	for i := uint32(0); i < colorLen; i++ {
		off := []int{offsets.PlayerAddrPtr + int(i)*4}
		pc, _ := gr.readUint32(playerColorsPtr, off)
		sc, _ := gr.readUint32(shadowColorsPtr, off)
		if i == 0 && pc != 4279308742 {
			return
		}
		if pc == 4278190080 {
			gr.rainbowColor = int(i)
		}
		colors[i] = [2]string{uint32ToColorHex(pc), uint32ToColorHex(sc)}
	}
	gr.playerColors = colors
	gr.colorsInitialized = true
	log.Printf("[reader] Loaded %d player colors", colorLen)
}

// readCurrentServer reads the current server string from game memory.
func (gr *GameReader) readCurrentServer() {
	ptr, err := gr.readAddr(gr.moduleBase, gr.offsets.ServerManagerCurrentServer)
	if err != nil {
		return
	}
	gr.currentServer = gr.readString(ptr, 50)
}

// ── Low-level memory helpers ────────────────────────────────────────────────

// readAddr resolves a full pointer chain including the last offset:
// it dereferences every step INCLUDING the last, returning the pointer value
// stored at (penultimate + lastOffset).  This matches the TypeScript
// readMemory('ptr'/'pointer') behaviour.
func (gr *GameReader) readAddr(base uintptr, offsets []int) (uintptr, error) {
	if gr.proc == nil || base == 0 || len(offsets) == 0 {
		return 0, nil
	}
	addr := base
	for _, off := range offsets {
		next, err := gr.readPtrAt(addr + uintptr(off))
		if err != nil {
			return 0, err
		}
		if next == 0 {
			return 0, nil
		}
		addr = next
	}
	return addr, nil
}

// readPointer walks all-but-last offsets and returns (penultimate_addr + last_offset).
// Used by scalar readers (readUint32, readFloat, readByte) which then read the
// value at the returned address.
func (gr *GameReader) readPointer(base uintptr, offsets []int) (uintptr, error) {
	if gr.proc == nil || base == 0 || len(offsets) == 0 {
		return 0, nil
	}
	addr := base
	for i := 0; i < len(offsets)-1; i++ {
		next, err := gr.readPtrAt(addr + uintptr(offsets[i]))
		if err != nil {
			return 0, err
		}
		if next == 0 {
			return 0, nil
		}
		addr = next
	}
	return addr + uintptr(offsets[len(offsets)-1]), nil
}

// readPointerDirect reads a pointer-sized value from the given absolute address.
func (gr *GameReader) readPointerDirect(addr uintptr) (uintptr, error) {
	if gr.proc.is64bit {
		buf, err := gr.proc.readBytes(addr, 8)
		if err != nil {
			return 0, err
		}
		return uintptr(binary.LittleEndian.Uint64(buf)), nil
	}
	buf, err := gr.proc.readBytes(addr, 4)
	if err != nil {
		return 0, err
	}
	return uintptr(binary.LittleEndian.Uint32(buf)), nil
}

// readPtrAt reads one pointer at the given address.
func (gr *GameReader) readPtrAt(addr uintptr) (uintptr, error) {
	return gr.readPointerDirect(addr)
}

// readUint32 resolves a pointer chain and reads a uint32.
func (gr *GameReader) readUint32(base uintptr, offsets []int) (uint32, error) {
	addr, err := gr.readPointer(base, offsets)
	if err != nil || addr == 0 {
		return 0, err
	}
	buf, err := gr.proc.readBytes(addr, 4)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(buf), nil
}

// readUint32Offset reads a uint32 at base+offset (single offset, no chain).
func (gr *GameReader) readUint32Offset(base uintptr, offset int) (uint32, error) {
	return gr.readUint32(base, []int{offset})
}

// readInt32 resolves a pointer chain and reads an int32.
func (gr *GameReader) readInt32(base uintptr, offsets []int) (int32, error) {
	v, err := gr.readUint32(base, offsets)
	return int32(v), err
}

// readInt32Offset reads an int32 at base+offset.
func (gr *GameReader) readInt32Offset(base uintptr, offset int) (int32, error) {
	return gr.readInt32(base, []int{offset})
}

// readInt32Direct reads an int32 from the given absolute address.
func (gr *GameReader) readInt32Direct(addr uintptr) (int32, error) {
	buf, err := gr.proc.readBytes(addr, 4)
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(buf)), nil
}

// readFloat resolves a pointer chain and reads a float32.
func (gr *GameReader) readFloat(base uintptr, offsets []int) float32 {
	v, err := gr.readUint32(base, offsets)
	if err != nil {
		return 0
	}
	return math.Float32frombits(v)
}

// readByte resolves a pointer chain and reads a single byte as uint32.
func (gr *GameReader) readByte(base uintptr, offsets []int) byte {
	addr, err := gr.readPointer(base, offsets)
	if err != nil || addr == 0 {
		return 0
	}
	buf, err := gr.proc.readBytes(addr, 1)
	if err != nil || len(buf) == 0 {
		return 0
	}
	return buf[0]
}

// offsetAddress walks a pointer chain and returns (penultimate address, last offset).
// Mirrors the TypeScript offsetAddress() method.
func (gr *GameReader) offsetAddress(base uintptr, offsets []int) (uintptr, int, error) {
	if len(offsets) == 0 {
		return base, 0, nil
	}
	addr := base
	for i := 0; i < len(offsets)-1; i++ {
		next, err := gr.readPtrAt(addr + uintptr(offsets[i]))
		if err != nil {
			return 0, 0, err
		}
		if next == 0 {
			return 0, 0, nil
		}
		addr = next
	}
	return addr, offsets[len(offsets)-1], nil
}

// readDictionary iterates over a Unity IL2CPP Dictionary, calling cb for each
// (key-pointer, value-pointer, index) entry.
func (gr *GameReader) readDictionary(
	address uintptr,
	maxLen int,
	cb func(kPtr, vPtr uintptr, index int),
) {
	var entriesOff, lenOff, entryStride, kvOff uintptr
	if gr.proc.is64bit {
		entriesOff = 0x18
		lenOff = 0x20
		entryStride = 0x18
		kvOff = 0x10
	} else {
		entriesOff = 0x0c
		lenOff = 0x10
		entryStride = 0x10
		kvOff = 0x0c
	}

	entriesPtr, _ := gr.readPointerDirect(address + entriesOff)
	lenBuf, err := gr.proc.readBytes(address+lenOff, 4)
	if err != nil {
		return
	}
	length := int(binary.LittleEndian.Uint32(lenBuf))
	if length > maxLen {
		length = maxLen
	}

	var headerSize uintptr
	if gr.proc.is64bit {
		headerSize = 0x20
	} else {
		headerSize = 0x10
	}

	for i := 0; i < length; i++ {
		offset := entriesPtr + headerSize + uintptr(i)*entryStride
		cb(offset, offset+kvOff, i)
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// hasLocal returns true if any player in the slice is local.
func hasLocal(players []types.Player) bool {
	for _, p := range players {
		if p.IsLocal {
			return true
		}
	}
	return false
}

// uint32ToColorHex converts a 32-bit RGBA color to a "#rrggbb" hex string.
func uint32ToColorHex(c uint32) string {
	r := (c >> 16) & 0xff
	g := (c >> 8) & 0xff
	b := c & 0xff
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// getInstalledMod detects which mod is installed based on the game path.
func getInstalledMod(gamePath string) types.Mod {
	lower := strings.ToLower(gamePath)
	if strings.Contains(lower, "?\\volume") {
		return types.ModList[0]
	}
	dir := filepath.Dir(gamePath)
	winhttp := filepath.Join(dir, "winhttp.dll")
	bepPlugins := filepath.Join(dir, "BepInEx", "plugins")
	if _, err := os.Stat(winhttp); err != nil {
		return types.ModList[0]
	}
	entries, err := os.ReadDir(bepPlugins)
	if err != nil {
		return types.ModList[0]
	}
	for _, e := range entries {
		name := e.Name()
		for _, mod := range types.ModList[1:] {
			if mod.DLLStartsWith != "" && strings.Contains(name, mod.DLLStartsWith) {
				return mod
			}
		}
	}
	return types.ModList[0]
}

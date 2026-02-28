package types

// Vector2 is a 2D world-space coordinate (same units as player X/Y).
type Vector2 struct {
	X float32
	Y float32
}

// GameState represents the current state of the Among Us game.
type GameState int

const (
	GameStateLobby      GameState = 0
	GameStateTasks      GameState = 1
	GameStateDiscussion GameState = 2
	GameStateMenu       GameState = 3
	GameStateUnknown    GameState = 4
)

func (g GameState) String() string {
	switch g {
	case GameStateLobby:
		return "LOBBY"
	case GameStateTasks:
		return "TASKS"
	case GameStateDiscussion:
		return "DISCUSSION"
	case GameStateMenu:
		return "MENU"
	default:
		return "UNKNOWN"
	}
}

// MapType identifies the Among Us map.
type MapType int

const (
	MapTypeTheSkeld      MapType = 0
	MapTypeMiraHQ        MapType = 1
	MapTypePolus         MapType = 2
	MapTypeTheSkeldApril MapType = 3
	MapTypeAirship       MapType = 4
	MapTypeFungle        MapType = 5
	MapTypeUnknown       MapType = 6
	MapTypeSubmerged     MapType = 105
)

func (m MapType) String() string {
	switch m {
	case MapTypeTheSkeld:
		return "The Skeld"
	case MapTypeMiraHQ:
		return "Mira HQ"
	case MapTypePolus:
		return "Polus"
	case MapTypeTheSkeldApril:
		return "The Skeld (April)"
	case MapTypeAirship:
		return "Airship"
	case MapTypeFungle:
		return "Fungle"
	case MapTypeSubmerged:
		return "Submerged"
	default:
		return "Unknown"
	}
}

// CameraLocation identifies a surveillance camera slot.
type CameraLocation int

const (
	CameraEast      CameraLocation = 0 // Engine Room (Polus/Airship)
	CameraCentral   CameraLocation = 1 // Vault
	CameraNortheast CameraLocation = 2 // Records
	CameraSouth     CameraLocation = 3 // Security
	CameraSouthWest CameraLocation = 4 // Cargo Bay
	CameraNorthWest CameraLocation = 5 // Meeting Room
	CameraSkeld     CameraLocation = 6
	CameraNone      CameraLocation = 7
)

// Player holds all data extracted for a single Among Us player.
type Player struct {
	Ptr          uintptr
	ID           uint32
	ClientID     uint32
	Name         string
	NameHash     int32
	ColorID      uint32
	ShiftedColor int32
	HatID        string
	PetID        string
	SkinID       string
	VisorID      string
	Disconnected bool
	IsImpostor   bool
	IsDead       bool
	InVent       bool
	IsLocal      bool
	IsDummy      bool
	Bugged       bool
	X            float32
	Y            float32
	ObjectPtr    uintptr
	TaskPtr      uintptr
}

// AmongUsState is the full game state snapshot emitted on every reader tick.
type AmongUsState struct {
	GameState          GameState
	OldGameState       GameState
	LobbyCode          string
	LobbyCodeInt       int32
	Players            []Player
	IsHost             bool
	ClientID           uint32
	HostID             uint32
	ComsSabotaged      bool
	CurrentCamera      CameraLocation
	Map                MapType
	LightRadius        float32
	LightRadiusChanged bool
	ClosedDoors        []int
	CurrentServer      string
	MaxPlayers         int
	Mod                ModsType
	OldMeetingHud      bool
}

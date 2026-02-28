package types

// ISignature describes a byte-pattern signature used to locate a structure
// in GameAssembly.dll at runtime.
type ISignature struct {
	Sig           string `json:"sig"`
	AddressOffset int    `json:"addressOffset"`
	PatternOffset int    `json:"patternOffset"`
}

// IOffsetsLookup is the top-level lookup.json fetched from the offsets repo.
// It maps a broadcastVersion integer (read from game memory) to the correct
// offsets file, and provides the signatures needed to detect that version.
type IOffsetsLookup struct {
	Patterns struct {
		X64 struct {
			BroadcastVersion ISignature `json:"broadcastVersion"`
		} `json:"x64"`
		X86 struct {
			BroadcastVersion ISignature `json:"broadcastVersion"`
		} `json:"x86"`
	} `json:"patterns"`
	Versions map[string]struct {
		Version        string `json:"version"`
		File           string `json:"file"`
		OffsetsVersion int    `json:"offsetsVersion"`
	} `json:"versions"`
}

// StructMemberType enumerates the primitive types supported by the player
// struct definition in the offsets JSON.
type StructMemberType string

const (
	StructTypeINT      StructMemberType = "INT"
	StructTypeINTBE    StructMemberType = "INT_BE"
	StructTypeUINT     StructMemberType = "UINT"
	StructTypeUINTBE   StructMemberType = "UINT_BE"
	StructTypeSHORT    StructMemberType = "SHORT"
	StructTypeSHORTBE  StructMemberType = "SHORT_BE"
	StructTypeUSHORT   StructMemberType = "USHORT"
	StructTypeUSHORTBE StructMemberType = "USHORT_BE"
	StructTypeFLOAT    StructMemberType = "FLOAT"
	StructTypeCHAR     StructMemberType = "CHAR"
	StructTypeBYTE     StructMemberType = "BYTE"
	StructTypeSKIP     StructMemberType = "SKIP"
)

// StructMember describes one field in the player raw-buffer layout.
type StructMember struct {
	Type StructMemberType `json:"type"`
	Skip *int             `json:"skip,omitempty"`
	Name string           `json:"name"`
}

// PlayerOffsets holds all the sub-offsets that describe where player data
// lives relative to the player object pointer.
type PlayerOffsets struct {
	IsLocal       []int `json:"isLocal"`
	LocalX        []int `json:"localX"`
	LocalY        []int `json:"localY"`
	RemoteX       []int `json:"remoteX"`
	RemoteY       []int `json:"remoteY"`
	RoleTeam      []int `json:"roleTeam"`
	NameText      []int `json:"nameText,omitempty"`
	CurrentOutfit []int `json:"currentOutfit"`
	Outfit        struct {
		ColorID    []int `json:"colorId"`
		PlayerName []int `json:"playerName"`
		HatID      []int `json:"hatId"`
		SkinID     []int `json:"skinId"`
		VisorID    []int `json:"visorId"`
	} `json:"outfit"`
	BufferLength int            `json:"bufferLength"`
	Offsets      []int          `json:"offsets"`
	InVent       []int          `json:"inVent"`
	ClientID     []int          `json:"clientId"`
	IsDummy      []int          `json:"isDummy"`
	Struct       []StructMember `json:"struct"`
}

// InnerNetClientOffsets holds offsets within the InnerNetClient object.
type InnerNetClientOffsets struct {
	Base           []int `json:"base"`
	NetworkAddress int   `json:"networkAddress"`
	NetworkPort    int   `json:"networkPort"`
	OnlineScene    int   `json:"onlineScene"`
	MainMenuScene  int   `json:"mainMenuScene"`
	GameMode       int   `json:"gameMode"`
	GameID         int   `json:"gameId"`
	HostID         int   `json:"hostId"`
	ClientID       int   `json:"clientId"`
	GameState      int   `json:"gameState"`
}

// OffsetSignatures groups all the signatures used during initialisation.
type OffsetSignatures struct {
	InnerNetClient     ISignature `json:"innerNetClient"`
	MeetingHud         ISignature `json:"meetingHud"`
	GameData           ISignature `json:"gameData"`
	ShipStatus         ISignature `json:"shipStatus"`
	MiniGame           ISignature `json:"miniGame"`
	Palette            ISignature `json:"palette"`
	PlayerControl      ISignature `json:"playerControl"`
	ConnectFunc        ISignature `json:"connectFunc"`
	FixedUpdateFunc    ISignature `json:"fixedUpdateFunc"`
	PingMessageString  ISignature `json:"pingMessageString"`
	ServerManager      ISignature `json:"serverManager"`
	ShowModStamp       ISignature `json:"showModStamp"`
	ModLateUpdate      ISignature `json:"modLateUpdate"`
	GameOptionsManager ISignature `json:"gameOptionsManager"`
}

// IOffsets is the full offsets structure loaded from the versioned JSON file.
// Array values are pointer-chains: [base_offset, next_offset, ..., final_offset].
type IOffsets struct {
	MeetingHud                      []int                 `json:"meetingHud"`
	ObjectCachePtr                  []int                 `json:"objectCachePtr"`
	MeetingHudState                 []int                 `json:"meetingHudState"`
	AllPlayersPtr                   []int                 `json:"allPlayersPtr"`
	AllPlayers                      []int                 `json:"allPlayers"`
	PlayerCount                     []int                 `json:"playerCount"`
	PlayerAddrPtr                   int                   `json:"playerAddrPtr"`
	ShipStatus                      []int                 `json:"shipStatus"`
	LightRadius                     []int                 `json:"lightRadius"`
	ShipStatusSystems               []int                 `json:"shipStatus_systems"`
	ShipStatusMap                   []int                 `json:"shipStatus_map"`
	ShipStatusAllDoors              []int                 `json:"shipstatus_allDoors"`
	DoorDoorID                      int                   `json:"door_doorId"`
	DoorIsOpen                      int                   `json:"door_isOpen"`
	DeconDoorUpperOpen              []int                 `json:"deconDoorUpperOpen"`
	DeconDoorLowerOpen              []int                 `json:"deconDoorLowerOpen"`
	HqHudCompletedConsoles          []int                 `json:"hqHudSystemType_CompletedConsoles"`
	HudOverrideIsActive             []int                 `json:"HudOverrideSystemType_isActive"`
	MiniGame                        []int                 `json:"miniGame"`
	PlanetSurveillanceCurrentCamera []int                 `json:"planetSurveillanceMinigame_currentCamera"`
	PlanetSurveillanceCamarasCount  []int                 `json:"planetSurveillanceMinigame_camarasCount"`
	SurveillanceFilteredRoomsCount  []int                 `json:"surveillanceMinigame_FilteredRoomsCount"`
	Palette                         []int                 `json:"palette"`
	PaletteShadowColor              []int                 `json:"palette_shadowColor"`
	PalettePlayerColor              []int                 `json:"palette_playercolor"`
	GameOptionsData                 []int                 `json:"gameoptionsData"`
	GameOptionsMapID                []int                 `json:"gameOptions_MapId"`
	GameOptionsMaxPlayers           []int                 `json:"gameOptions_MaxPLayers"`
	ConnectFunc                     int                   `json:"connectFunc"`
	FixedUpdateFunc                 int                   `json:"fixedUpdateFunc"`
	ShowModStampFunc                int                   `json:"showModStampFunc"`
	ModLateUpdateFunc               int                   `json:"modLateUpdateFunc"`
	PingMessageString               int                   `json:"pingMessageString"`
	ServerManagerCurrentServer      []int                 `json:"serverManager_currentServer"`
	InnerNetClient                  InnerNetClientOffsets `json:"innerNetClient"`
	Player                          PlayerOffsets         `json:"player"`
	Signatures                      OffsetSignatures      `json:"signatures"`
	OldMeetingHud                   bool                  `json:"oldMeetingHud"`
	DisableWriting                  bool                  `json:"disableWriting"`
	NewGameOptions                  bool                  `json:"newGameOptions"`
}

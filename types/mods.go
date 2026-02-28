package types

// ModsType identifies which Among Us mod (if any) is installed.
type ModsType string

const (
	ModNone          ModsType = "NONE"
	ModTownOfUsMira  ModsType = "TOWN_OF_US_MIRA"
	ModTownOfUs      ModsType = "TOWN_OF_US"
	ModTheOtherRoles ModsType = "THE_OTHER_ROLES"
	ModLasMonjas     ModsType = "LAS_MONJAS"
	ModOther         ModsType = "OTHER"
)

// Mod describes a known mod and the DLL prefix used to detect it.
type Mod struct {
	ID            ModsType
	Label         string
	DLLStartsWith string // prefix of the BepInEx plugin DLL filename
}

// ModList is the ordered list of known mods (first entry = no mod).
var ModList = []Mod{
	{ID: ModNone, Label: "None"},
	{ID: ModTownOfUsMira, Label: "Town of Us: Mira", DLLStartsWith: "TownOfUsMira"},
	{ID: ModTownOfUs, Label: "Town of Us: Reactivated", DLLStartsWith: "TownOfUs"},
	{ID: ModTheOtherRoles, Label: "The Other Roles", DLLStartsWith: "TheOtherRoles"},
	{ID: ModLasMonjas, Label: "Las Monjas", DLLStartsWith: "LasMonjas"},
	{ID: ModOther, Label: "Other"},
}

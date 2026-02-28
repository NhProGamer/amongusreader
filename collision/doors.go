package collision

import "github.com/NhProGamer/AmongUsReader/types"

// doorPaths holds raw SVG path strings for each door by (MapType, doorID),
// ported directly from ColliderMap.ts.
var doorPaths = map[types.MapType]map[int]string{
	types.MapTypeMiraHQ: {
		0: "M 44.942 37.086 H 47.27",
		1: "M 44.942 30.499 H 47.27",
	},
	types.MapTypePolus: {
		0:  "M 51.257 48.531 V 50.205", // right electrical door
		1:  "M 48.14 50.544 H 46.811",  // electrical fence door
		2:  "M 44.751 53.131 H 46.111", // electrical door to O2
		3:  "M 44.798 58.095 H 46.244", // O2 door to electrical
		4:  "M 45.244 61.962 H 46.577", // O2 to outside
		5:  "M 52.257 60.719 H 53.791", // Weapons door
		6:  "M 50.113 58.923 H 51.646", // Communications door
		7:  "M 68.703 56.136 V 57.763", // office vitals door
		8:  "M 57.403 60.87 V 62.541",  // office left door
		9:  "M 65.938 48.445 H 67.311", // door to drill
		10: "M 64.738 48.655 V 50.384", // door outside to med
		11: "M 57.193 49.972 V 51.668", // storage door
		12: "M 65.475 63.639 V 65.376", // decon door office→spec
		13: "M 63.226 63.121 H 64.575", // decon door spec→office
		14: "M 77.996 50.401 V 48.711", // decon door med→spec
		15: "M 78.363 50.967 H 79.756", // decon door spec→med
	},
	types.MapTypeTheSkeld: {
		0:  "M 45.059 37.568 V 39.744", // cafeteria → weapons
		1:  "M 34.786 55.353 V 53.101", // storage → electrical
		2:  "M 25.3 39.787 V 37.568",   // upper engine ← medbay hallway
		3:  "M 38.371 44.717 H 40.207", // cafeteria → Admin hallway
		4:  "M 22.196 48.663 H 24.169", // lower engine ← security hallway
		5:  "M 22.196 41.945 H 23.989", // upper engine ← security hallway
		6:  "M 25.154 44.051 V 46.183", // security
		7:  "M 38.371 48.231 H 40.207", // storage → Admin
		8:  "M 33.649 37.568 V 39.787", // cafeteria → medbay hallway
		9:  "M 29.628 53.101 H 31.333", // electrical
		10: "M 29.977 39.787 H 31.77",  // medbay
		11: "M 25.412 52.405 V 50.274", // lower engine ← electrical hallway
		12: "M 41.081 52.971 V 50.839", // storage → shields
	},
	types.MapTypeAirship: {
		0:  "M 23.8376 41.7526 V 39.96",   // Comms Left
		1:  "M 31.1709 39.96 V 41.9155",   // Comms Right
		2:  "M 27.5694 39.6096 H 25.8339", // Comms Top
		3:  "M 28.295 41.9155 H 29.833",   // Comms Bottom
		4:  "M 38.428 32.5452 H 39.753",   // Brig Bottom
		5:  "M 36.3043 31.9748 V 30.424",  // Brig Left
		6:  "M 42.6598 30.224 V 32.012",   // Brig Right
		7:  "M 31.122 46.3563 V 48.0104",  // Armory Right
		8:  "M 31.2524 51.1229 V 52.7363", // Kitchen Left
		9:  "M 38.4228 50.8785 V 52.8341", // Kitchen Right
		10: "M 44.1346 39.0229 V 40.8155", // Hallway left
		11: "M 57.426 39.117 V 40.8155",   // Hallway right
		12: "M 56.418 31.3637 V 29.788",   // Records Left
		13: "M 64.008 29.7748 V 38.1674",  // Records Right
		14: "M 59.078 34.0118 H 60.578",   // Records Bottom
		15: "M 68.5709 33.197 H 70.0376",  // Bathroom door 1
		16: "M 70.005 33.197 H 71.4228",   // Bathroom door 2
		17: "M 71.5857 33.197 H 72.9709",  // Bathroom door 3
		18: "M 73.1339 33.197 H 74.4376",  // Bathroom door 4
		19: "M 61.2946 47.66 V 49.1674",   // Medical Left
		20: "M 71.708 44.3274 H 73.3457",  // Medical Top
	},
	types.MapTypeFungle: {
		0: "M 62.95 27.87 H 65.34", // Horizontal Comms
		1: "M 58.53 27.51 V 25.63", // Vertical Comms
		2: "M 23.51 45.57 H 25.44", // Kitchen
		3: "M 34.65 47.6 H 36.84",  // Laboratory
		4: "M 50.9 38.43 V 35.82",  // Lookout
		5: "M 51.38 33.62 H 53.88", // MiningPit
		6: "M 59.52 47.91 V 45.62", // Reactor
		7: "M 38.31 36.76 V 34.37", // Storage
	},
}

// DoorSegments returns the pre-parsed wall segments for a single door on the
// given map.  Returns nil if the door or map is not found.
func DoorSegments(mapType types.MapType, doorID int) []Segment {
	if mapType == types.MapTypeTheSkeldApril {
		mapType = types.MapTypeTheSkeld
	}
	doors, ok := doorPaths[mapType]
	if !ok {
		return nil
	}
	path, ok := doors[doorID]
	if !ok {
		return nil
	}
	segs, err := ParseSVGPath(path)
	if err != nil {
		return nil
	}
	return segs
}

// RawDoorPath returns the raw SVG path string for a single door.
// Returns ("", false) if not found.
func RawDoorPath(mapType types.MapType, doorID int) (string, bool) {
	if mapType == types.MapTypeTheSkeldApril {
		mapType = types.MapTypeTheSkeld
	}
	doors, ok := doorPaths[mapType]
	if !ok {
		return "", false
	}
	p, ok := doors[doorID]
	return p, ok
}

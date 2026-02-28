package collision

import "github.com/NhProGamer/AmongUsReader/types"

// PoseCollide reports whether the line of sight from world-space position p1 to
// p2 is blocked by any wall collider or closed door on the given map.
//
// This is a direct port of the TypeScript poseCollide() function from
// ColliderMap.ts.  The SVG coordinate transform applied internally is:
//
//	svgX = worldX + 40
//	svgY = 40 - worldY
func PoseCollide(p1, p2 types.Vector2, mapType types.MapType, closedDoors []int) bool {
	// April Fool's map mirrors X then delegates to The Skeld.
	if mapType == types.MapTypeTheSkeldApril {
		p1.X = -p1.X
		p2.X = -p2.X
		mapType = types.MapTypeTheSkeld
	}

	if mapType == types.MapTypeUnknown {
		return false
	}

	// Build the query segment in SVG space.
	query := Segment{
		A: worldToSVG(p1),
		B: worldToSVG(p2),
	}

	// Check all wall colliders.
	for _, pathSegs := range ColliderSegments(mapType) {
		if PathIntersectsSegment(pathSegs, query) {
			return true
		}
	}

	// Check closed doors.
	for _, doorID := range closedDoors {
		doorSegs := DoorSegments(mapType, doorID)
		if PathIntersectsSegment(doorSegs, query) {
			return true
		}
	}

	return false
}

// worldToSVG converts a game-world coordinate to SVG space.
//
//	svgX = worldX + 40
//	svgY = 40 - worldY
func worldToSVG(v types.Vector2) Point {
	return Point{
		X: float64(v.X) + 40,
		Y: 40 - float64(v.Y),
	}
}

// SVGToWorld is the inverse transform — useful for rasterising walls onto a
// world-space grid:
//
//	worldX = svgX - 40
//	worldY = 40 - svgY
func SVGToWorld(p Point) types.Vector2 {
	return types.Vector2{
		X: float32(p.X - 40),
		Y: float32(40 - p.Y),
	}
}

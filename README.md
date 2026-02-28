# AmongUsReader

A pure Go library for reading live Among Us game state from process memory.

Supports Windows (native) and Linux (Wine/Proton). No root or elevated privileges required.

## Requirements

- Go 1.21+
- Among Us running natively on Windows, or under Wine/Proton on Linux

## Installation

```sh
go get github.com/NhProGamer/AmongUsReader
```

## Quick start — poll game state

`reader.Start()` returns a channel that emits a new `types.AmongUsState` snapshot
at 5 Hz whenever Among Us is running. The channel is closed on fatal error.

```go
package main

import (
    "fmt"

    "github.com/NhProGamer/AmongUsReader/reader"
    "github.com/NhProGamer/AmongUsReader/types"
)

func main() {
    for state := range reader.Start() {
        fmt.Printf("[%s] %s — %d player(s)\n",
            state.Map,
            state.GameState,
            len(state.Players),
        )
        for _, p := range state.Players {
            role := "crew"
            if p.IsImpostor {
                role = "IMPOSTOR"
            }
            fmt.Printf("  %-20s  (%.2f, %.2f)  %s\n", p.Name, p.X, p.Y, role)
        }
    }
}
```

## Key types

### `types.AmongUsState`

| Field           | Type             | Description                            |
| --------------- | ---------------- | -------------------------------------- |
| `GameState`     | `GameState`      | `Lobby`, `Tasks`, `Discussion`, `Menu` |
| `LobbyCode`     | `string`         | 4–6 letter code, or `"MENU"`           |
| `Players`       | `[]Player`       | All visible players                    |
| `Map`           | `MapType`        | Active map                             |
| `IsHost`        | `bool`           | Local client is host                   |
| `ComsSabotaged` | `bool`           | Communications sabotaged               |
| `LightRadius`   | `float32`        | Local player's vision radius           |
| `ClosedDoors`   | `[]int`          | Door indices currently closed          |
| `CurrentCamera` | `CameraLocation` | Camera slot in use (or `CameraNone`)   |
| `CurrentServer` | `string`         | Server region string                   |

### `types.Player`

| Field          | Type      | Description                |
| -------------- | --------- | -------------------------- |
| `Name`         | `string`  | Display name               |
| `X`, `Y`       | `float32` | World-space position       |
| `ColorID`      | `uint32`  | Colour slot index          |
| `IsImpostor`   | `bool`    |                            |
| `IsDead`       | `bool`    |                            |
| `InVent`       | `bool`    | Currently hiding in a vent |
| `IsLocal`      | `bool`    | This is the local player   |
| `Disconnected` | `bool`    |                            |

### `types.MapType`

```go
MapTypeTheSkeld      // 0
MapTypeMiraHQ        // 1
MapTypePolus         // 2
MapTypeTheSkeldApril // 3  (April Fools — mirrors X)
MapTypeAirship       // 4
MapTypeFungle        // 5
MapTypeSubmerged     // 105
```

### `types.GameState`

```go
GameStateLobby      // 0
GameStateTasks      // 1
GameStateDiscussion // 2
GameStateMenu       // 3
```

### `types.Vector2`

```go
type Vector2 struct {
    X float32
    Y float32
}
```

Used as input to the collision package.

---

## Collision package

`collision` provides line-of-sight checks and raw wall/door geometry for all
maps, ported directly from `ColliderMap.ts`.

### Coordinate system

```
svgX = worldX + 40
svgY = 40 - worldY    // Y is flipped
```

`SVGToWorld` and `worldToSVG` convert between the two spaces internally.
You only ever deal in world coordinates when calling `PoseCollide`.

### Line-of-sight check

```go
import (
    "github.com/NhProGamer/AmongUsReader/collision"
    "github.com/NhProGamer/AmongUsReader/types"
)

blocked := collision.PoseCollide(
    types.Vector2{X: p1.X, Y: p1.Y},
    types.Vector2{X: p2.X, Y: p2.Y},
    state.Map,
    state.ClosedDoors,
)
```

Returns `true` if the straight line from `p1` to `p2` is blocked by a wall or
a closed door on the given map.

### Raw geometry — custom map rendering

```go
// All wall segments for a map (SVG space).
wallPaths := collision.ColliderSegments(types.MapTypeTheSkeld)
for _, path := range wallPaths {
    for _, seg := range path {
        // seg.A, seg.B are collision.Point{X, Y float64} in SVG space.
        // Convert to world space:
        a := collision.SVGToWorld(seg.A)
        b := collision.SVGToWorld(seg.B)
        drawLine(a.X, a.Y, b.X, b.Y)
    }
}

// Door segments (SVG space) — door IDs match indices in state.ClosedDoors.
for _, doorID := range state.ClosedDoors {
    for _, seg := range collision.DoorSegments(state.Map, doorID) {
        a := collision.SVGToWorld(seg.A)
        b := collision.SVGToWorld(seg.B)
        drawLine(a.X, a.Y, b.X, b.Y)
    }
}
```

### SVG path parser

If you need to parse raw SVG path strings (from `ColliderMap.ts` or similar):

```go
segments, err := collision.ParseSVGPath("M 10 20 H 30 V 40 Z")
```

Supports `M L H V Z` commands. Cubic bezier curves (`C`) are silently skipped
(they only appear in Submerged, which is untested upstream).

---

## Map world-coordinate bounds

Computed from the actual SVG collider paths:

| Map       | X           | Y           |
| --------- | ----------- | ----------- |
| The Skeld | `[-25, 21]` | `[-20, 9]`  |
| Mira HQ   | `[-13, 31]` | `[-6, 28]`  |
| Polus     | `[-2, 43]`  | `[-28, 3]`  |
| Airship   | `[-27, 42]` | `[-19, 20]` |
| Fungle    | `[-23, 28]` | `[-17, 17]` |
| Submerged | `[-18, 18]` | `[-46, 37]` |

---

## Notes

- Offsets are fetched at startup from the BetterCrewLink offset CDN. An internet
  connection is required on first attach.
- The reader auto-detects 32-bit vs 64-bit Among Us builds.
- Mod detection (TheOtherRoles, etc.) is automatic based on the game path.

---

## Acknowledgements

This library would not exist without the following projects:
- **[BetterCrewLink](https://github.com/OhMyGuus/BetterCrewLink)** by [OhMyGuus](https://github.com/OhMyGuus)
  — The memory reading logic, camera detection, collision data, and offset structure
  are all ported directly from this project.
- **[CrewLink](https://github.com/ottomated/CrewLink)** by [ottomated](https://github.com/ottomated)
  — The original proximity voice chat mod for Among Us, and the foundation on which
  BetterCrewLink is built.
- **[BetterCrewlink-Offsets](https://github.com/OhMyGuus/BetterCrewlink-Offsets)** by [OhMyGuus](https://github.com/OhMyGuus)
  — The versioned memory offset database fetched at runtime to support every
  Among Us release.
  
---

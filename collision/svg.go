// Package collision provides SVG path parsing and line-segment intersection
// tests used to determine whether two game-world positions are occlided by a
// wall or a closed door.
package collision

import (
	"fmt"
	"strconv"
	"strings"
)

// Point is a 2-D coordinate in SVG space (svgX = worldX+40, svgY = 40-worldY).
type Point struct{ X, Y float64 }

// Segment is a directed line segment from A to B.
type Segment struct{ A, B Point }

// ParseSVGPath parses a subset of SVG path data (M, H, V, L, Z commands) and
// returns the list of line segments it describes.  Cubic bezier curves (C) are
// silently skipped because they only appear in the Submerged map which is
// marked "not tested" in the original TypeScript source.
func ParseSVGPath(d string) ([]Segment, error) {
	tokens, err := tokenise(d)
	if err != nil {
		return nil, err
	}

	var segments []Segment
	var cur, start Point
	i := 0

	for i < len(tokens) {
		cmd := tokens[i]
		i++
		switch cmd {
		case "M":
			x, y, n, e := consume2(tokens, i)
			if e != nil {
				return nil, fmt.Errorf("M: %w", e)
			}
			cur = Point{x, y}
			start = cur
			i += n
			// Consume additional coordinate pairs as implicit L
			for i < len(tokens) && isNum(tokens[i]) {
				x, y, n2, e2 := consume2(tokens, i)
				if e2 != nil {
					break
				}
				next := Point{x, y}
				segments = append(segments, Segment{cur, next})
				cur = next
				i += n2
			}
		case "L":
			x, y, n, e := consume2(tokens, i)
			if e != nil {
				return nil, fmt.Errorf("L: %w", e)
			}
			next := Point{x, y}
			segments = append(segments, Segment{cur, next})
			cur = next
			i += n
			// Implicit L repetition
			for i < len(tokens) && isNum(tokens[i]) {
				x, y, n2, e2 := consume2(tokens, i)
				if e2 != nil {
					break
				}
				next2 := Point{x, y}
				segments = append(segments, Segment{cur, next2})
				cur = next2
				i += n2
			}
		case "H":
			x, n, e := consume1(tokens, i)
			if e != nil {
				return nil, fmt.Errorf("H: %w", e)
			}
			next := Point{x, cur.Y}
			segments = append(segments, Segment{cur, next})
			cur = next
			i += n
			for i < len(tokens) && isNum(tokens[i]) {
				x, n2, e2 := consume1(tokens, i)
				if e2 != nil {
					break
				}
				next2 := Point{x, cur.Y}
				segments = append(segments, Segment{cur, next2})
				cur = next2
				i += n2
			}
		case "V":
			y, n, e := consume1(tokens, i)
			if e != nil {
				return nil, fmt.Errorf("V: %w", e)
			}
			next := Point{cur.X, y}
			segments = append(segments, Segment{cur, next})
			cur = next
			i += n
			for i < len(tokens) && isNum(tokens[i]) {
				y, n2, e2 := consume1(tokens, i)
				if e2 != nil {
					break
				}
				next2 := Point{cur.X, y}
				segments = append(segments, Segment{cur, next2})
				cur = next2
				i += n2
			}
		case "Z", "z":
			if cur != start {
				segments = append(segments, Segment{cur, start})
			}
			cur = start
		case "C", "c", "S", "s", "Q", "q", "T", "t", "A", "a":
			// Skip curves — consume numbers until next command letter.
			for i < len(tokens) && isNum(tokens[i]) {
				i++
			}
		default:
			return nil, fmt.Errorf("unknown SVG command %q", cmd)
		}
	}
	return segments, nil
}

// SegmentsIntersect reports whether segments s1 and s2 share any interior or
// endpoint in common, using the standard cross-product parameterisation.
func SegmentsIntersect(s1, s2 Segment) bool {
	// We need t ∈ [0,1] and u ∈ [0,1] for the intersection to be within both
	// segments.
	dx1 := s1.B.X - s1.A.X
	dy1 := s1.B.Y - s1.A.Y
	dx2 := s2.B.X - s2.A.X
	dy2 := s2.B.Y - s2.A.Y

	denom := dx1*dy2 - dy1*dx2
	if abs64(denom) < 1e-10 {
		// Parallel (or coincident) — treat as non-intersecting for audio purposes.
		return false
	}

	ox := s2.A.X - s1.A.X
	oy := s2.A.Y - s1.A.Y

	t := (ox*dy2 - oy*dx2) / denom
	u := (ox*dy1 - oy*dx1) / denom

	return t >= 0 && t <= 1 && u >= 0 && u <= 1
}

// PathIntersectsSegment reports whether any segment in the parsed path
// intersects the query segment q.
func PathIntersectsSegment(path []Segment, q Segment) bool {
	for _, s := range path {
		if SegmentsIntersect(s, q) {
			return true
		}
	}
	return false
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// tokenise splits an SVG path data string into command letters and numeric
// tokens (handling both space and comma separators, and sign-attached numbers).
func tokenise(d string) ([]string, error) {
	d = strings.TrimSpace(d)
	var tokens []string
	i := 0
	for i < len(d) {
		c := d[i]
		if c == ' ' || c == ',' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}
		if isLetter(c) {
			tokens = append(tokens, string(c))
			i++
			continue
		}
		// Number: optional sign, digits, optional decimal
		j := i
		if j < len(d) && (d[j] == '-' || d[j] == '+') {
			j++
		}
		start := j
		for j < len(d) && (d[j] >= '0' && d[j] <= '9') {
			j++
		}
		if j < len(d) && d[j] == '.' {
			j++
			for j < len(d) && (d[j] >= '0' && d[j] <= '9') {
				j++
			}
		}
		if j == start {
			return nil, fmt.Errorf("unexpected character %q at position %d", c, i)
		}
		tokens = append(tokens, d[i:j])
		i = j
	}
	return tokens, nil
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isNum(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func consume1(tokens []string, i int) (float64, int, error) {
	if i >= len(tokens) {
		return 0, 0, fmt.Errorf("expected number, got EOF")
	}
	v, err := strconv.ParseFloat(tokens[i], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("expected number, got %q: %w", tokens[i], err)
	}
	return v, 1, nil
}

func consume2(tokens []string, i int) (float64, float64, int, error) {
	x, nx, err := consume1(tokens, i)
	if err != nil {
		return 0, 0, 0, err
	}
	y, ny, err := consume1(tokens, i+nx)
	if err != nil {
		return 0, 0, 0, err
	}
	return x, y, nx + ny, nil
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

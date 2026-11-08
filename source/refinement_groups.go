package main

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// A recommendation is a draft, never an ACT operation. Ansys owns the actual
// geometry scopes and mesh controls. Area alone cannot establish function.
type NamedAdvice struct {
	Name          string
	MM            float64
	Eligible      bool
	Reason        string
	Count         int
	MedianAreaMM2 float64
}
type SizeBucket struct {
	Key     string
	Label   string
	MM      float64
	Members []string
}
type NamedListRow struct {
	Group int
	Name  string // empty indicates an expandable group header
}

var sizeTokenRE = regexp.MustCompile(`(?i)([-+]?[0-9]+(?:\.[0-9]+)?)\s*(\[\s*mm\s*\]|mm|\[\s*m\s*\]|\s*m\b)`)

func displayedGlobalMM(s *Snapshot, fallback float64) float64 {
	if s == nil || s.Mesh == nil {
		return fallback
	}
	raw := fmt.Sprint(s.Mesh["ElementSize"])
	m := sizeTokenRE.FindStringSubmatch(raw)
	if len(m) == 0 {
		return fallback
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil || n <= 0 {
		return fallback
	}
	unit := strings.ToLower(strings.ReplaceAll(m[2], " ", ""))
	if unit == "m" || unit == "[m]" {
		n *= 1000
	}
	if n >= 0.5 && n <= 500 {
		return n
	}
	return fallback
}
func geometryToMM(s *Snapshot) float64 {
	if s == nil {
		return 1
	}
	switch strings.ToLower(strings.TrimSpace(s.GeometryUnit)) {
	case "m", "meter", "meters":
		return 1000
	case "cm", "centimeter", "centimeters":
		return 10
	case "um", "micrometer", "micron":
		return .001
	case "inch", "in":
		return 25.4
	case "ft", "foot":
		return 304.8
	default:
		return 1
	}
}
func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	i := len(values) / 2
	if len(values)%2 == 1 {
		return values[i]
	}
	return (values[i-1] + values[i]) / 2
}
func minmax(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
func roundedSize(mm float64) float64 { return math.Round(mm*10) / 10 }

func recommendNamedSizes(s *Snapshot, referenceMM float64) []NamedAdvice {
	if s == nil {
		return nil
	}
	global := displayedGlobalMM(s, referenceMM)
	if global <= 0 {
		global = 8
	}
	faceByID := map[int]Face{}
	for _, f := range s.Faces {
		faceByID[f.ID] = f
	}
	factor := geometryToMM(s)
	result := make([]NamedAdvice, 0, len(s.Named))
	for _, n := range s.Named {
		name := strings.TrimSpace(n.Name)
		if name == "" || strings.EqualFold(name, "inlet") || strings.EqualFold(name, "outlet") || len(n.IDs) == 0 {
			continue
		}
		values := make([]float64, 0, len(n.IDs))
		curveCount := 0
		valid := 0
		for _, id := range n.IDs {
			f, ok := faceByID[id]
			if !ok || f.Area <= 0 {
				continue
			}
			val := f.Area * factor * factor
			if math.IsNaN(val) || math.IsInf(val, 0) || val <= 0 {
				continue
			}
			values = append(values, val)
			valid++
			typ := strings.ToLower(f.Type)
			if strings.Contains(typ, "cyl") || strings.Contains(typ, "curve") || strings.Contains(typ, "cone") || strings.Contains(typ, "tor") || strings.Contains(typ, "spher") {
				curveCount++
			}
		}
		if valid == 0 {
			continue
		}
		area := median(values)
		// A conservative starting heuristic, not a verified minimum cell size.
		proposal := minmax(math.Sqrt(area)*0.38, 0.5, global)
		if curveCount > valid/2 {
			proposal = math.Min(proposal, global*.65)
		}
		lname := strings.ToLower(name)
		if strings.Contains(lname, "wafer") || strings.Contains(name, "晶圆") || strings.Contains(lname, "4inch") {
			proposal = math.Min(proposal, global*.45)
		}
		if valid >= 50 && curveCount == 0 && area > global*global {
			proposal = math.Min(proposal, global*.65) // repeated large faces might still be significant
		}
		proposal = roundedSize(minmax(proposal, 0.5, global))
		if proposal < 0.5 {
			proposal = 0.5
		}
		// Without functional evidence, only simple small cylindrical groups
		// are candidates for automatic activation. Others require review.
		confidentSmall := strings.HasPrefix(name, "AM_") && curveCount > valid*3/4 &&
			area <= global*global && proposal <= global*.5
		result = append(result, NamedAdvice{Name: name, MM: proposal, Eligible: confidentSmall,
			Reason: fmt.Sprintf("面积中位数 %.2f mm²；%d/%d 个解析曲面；尺寸仅为初步建议", area, curveCount, valid),
			Count:  valid, MedianAreaMM2: area})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].MM != result[j].MM {
			return result[i].MM < result[j].MM
		}
		return result[i].Name < result[j].Name
	})
	return result
}
func groupAdvice(advice []NamedAdvice, sizes map[string]float64) []SizeBucket {
	groups := []SizeBucket{}
	for _, a := range advice {
		mm := a.MM
		if v, ok := sizes[a.Name]; ok && v >= 0.35 {
			mm = v
		}
		target := -1
		for i := range groups {
			if math.Abs(groups[i].MM-mm)/math.Max(groups[i].MM, mm) <= 0.24 {
				target = i
				break
			}
		}
		if target == -1 {
			groups = append(groups, SizeBucket{Key: fmt.Sprintf("group_%d", len(groups)+1), MM: mm})
			target = len(groups) - 1
		}
		groups[target].Members = append(groups[target].Members, a.Name)
	}
	for i := range groups {
		n := groups[i].MM
		label := "局部细化组"
		if n <= 2 {
			label = "细网格组"
		} else if n <= 5 {
			label = "中网格组"
		} else {
			label = "较粗网格／待审核组"
		}
		groups[i].Label = fmt.Sprintf("%s %d · %.2g mm · %d 个命名", label, i+1, n, len(groups[i].Members))
	}
	return groups
}
func advisedActivation(advice []NamedAdvice, s *Snapshot) map[string]float64 {
	result := map[string]float64{}
	// Conservative: overlapping scopes require explicit review before publish.
	byName := map[string][]int{}
	if s != nil {
		for _, n := range s.Named {
			byName[n.Name] = n.IDs
		}
	}
	used := map[int]bool{}
	for _, a := range advice {
		if !a.Eligible {
			continue
		}
		overlapping := false
		for _, id := range byName[a.Name] {
			if used[id] {
				overlapping = true
				break
			}
		}
		if overlapping {
			continue
		}
		result[a.Name] = a.MM
		for _, id := range byName[a.Name] {
			used[id] = true
		}
	}
	return result
}

// Preview is deliberately limited to newly requested Named Selections.
// Existing manual Sizing is validated in the real ACT preflight at publish time.
// This is a surface complexity proxy, not cell count or RAM consumption.
func previewSurfaceBudget(s *Snapshot, namedSizes map[string]float64) (float64, int, string) {
	if s == nil {
		return 0, 0, ""
	}
	factor := geometryToMM(s)
	area := map[int]float64{}
	for _, face := range s.Faces {
		if face.Area > 0 && !math.IsNaN(face.Area) && !math.IsInf(face.Area, 0) {
			area[face.ID] = face.Area * factor * factor
		}
	}
	uniqueSize := map[int]float64{}
	overlap := 0
	topName := ""
	topWeight := 0.0
	for _, n := range s.Named {
		size, ok := namedSizes[n.Name]
		if !ok || size <= 0 {
			continue
		}
		weight := 0.0
		for _, id := range n.IDs {
			a, valid := area[id]
			if !valid {
				continue
			}
			weight += a / (size * size)
			if old, exists := uniqueSize[id]; exists {
				overlap++
				if size < old {
					uniqueSize[id] = size
				}
			} else {
				uniqueSize[id] = size
			}
		}
		if weight > topWeight {
			topWeight = weight
			topName = n.Name
		}
	}
	total := 0.0
	for id, size := range uniqueSize {
		total += area[id] / (size * size)
	}
	return total, overlap, topName
}

// setBucketEnabled changes enablement without treating the group-size editor as
// an implicit overwrite. Off preserves each member's last individually edited
// millimeter size; on restores it (or uses the current suggestion).
func setBucketEnabled(active, paused map[string]float64, members []string, advice []NamedAdvice, defaultMM float64, enabled bool) {
	recommended := map[string]float64{}
	for _, item := range advice {
		recommended[item.Name] = item.MM
	}
	for _, name := range members {
		if !enabled {
			if mm, ok := active[name]; ok {
				paused[name] = mm
			}
			delete(active, name)
			continue
		}
		if _, ok := active[name]; ok {
			continue
		}
		mm := paused[name]
		if mm <= 0 {
			mm = recommended[name]
		}
		if mm <= 0 {
			mm = defaultMM
		}
		if mm > 0 {
			active[name] = mm
		}
	}
}

// Updating a grouped size changes only the stored size, never its enablement.
// This is also used when every member is disabled, so re-enabling later restores
// the user's 8 mm (or other) exact override rather than the recommendation.
func setBucketSizePreservingSelection(active, paused map[string]float64, members []string, mm float64) {
	for _, name := range members {
		if _, enabled := active[name]; enabled {
			active[name] = mm
		} else {
			paused[name] = mm
		}
	}
}

func bucketEnabledCount(active map[string]float64, members []string) int {
	n := 0
	for _, name := range members {
		if _, ok := active[name]; ok {
			n++
		}
	}
	return n
}

func preservePausedNamedSizes(previous map[string]float64, oldIdentity, newIdentity string, names []Named) map[string]float64 {
	if oldIdentity == "" || oldIdentity != newIdentity {
		return map[string]float64{}
	}
	available := make(map[string]bool, len(names))
	for _, n := range names {
		if len(n.IDs) != 0 {
			available[n.Name] = true
		}
	}
	result := map[string]float64{}
	for name, size := range previous {
		if available[name] && size > 0 {
			result[name] = size
		}
	}
	return result
}

// rebindNamedSettingsByScope preserves user-edited per-name sizes across a
// read-only rescan after Chinese/manual renames. It never guesses by display
// name or area. A rename must have one unique old and one unique new Named
// Selection with exactly the same sorted, non-empty face-ID scope.
func rebindNamedSettingsByScope(previous map[string]float64, oldNamed, newNamed []Named) (map[string]float64, map[string]string) {
	result := map[string]float64{}
	redirects := map[string]string{}
	oldScopes := map[string]string{}
	oldCounts := map[string]int{}
	newScopes := map[string]string{}
	newCounts := map[string]int{}
	newNamesByScope := map[string][]string{}
	oldNamesByScope := map[string][]string{}
	for _, n := range oldNamed {
		k := namedScopeKey(n.IDs)
		if k != "" {
			oldScopes[n.Name] = k
			oldCounts[k]++
			oldNamesByScope[k] = append(oldNamesByScope[k], n.Name)
		}
	}
	for _, n := range newNamed {
		k := namedScopeKey(n.IDs)
		if k != "" {
			newScopes[n.Name] = k
			newCounts[k]++
			newNamesByScope[k] = append(newNamesByScope[k], n.Name)
		}
	}
	// Existing exact names always win. Live Meshing scoping is authoritative.
	for name, mm := range previous {
		if mm > 0 && newScopes[name] != "" {
			result[name] = mm
		}
	}
	for oldName, mm := range previous {
		if mm <= 0 || newScopes[oldName] != "" {
			continue
		}
		k := oldScopes[oldName]
		if k == "" || oldCounts[k] != 1 || newCounts[k] != 1 {
			continue
		}
		newName := newNamesByScope[k][0]
		if strings.EqualFold(newName, "inlet") || strings.EqualFold(newName, "outlet") {
			continue
		}
		if _, occupied := result[newName]; occupied {
			continue
		}
		result[newName] = mm
		redirects[oldName] = newName
	}
	return result, redirects
}

func namedScopeKey(ids []int) string {
	if len(ids) == 0 {
		return ""
	}
	copyIDs := append([]int(nil), ids...)
	sort.Ints(copyIDs)
	var b strings.Builder
	prev := 0
	for i, n := range copyIDs {
		if i > 0 && n == prev {
			continue
		}
		b.WriteString(strconv.Itoa(n))
		b.WriteByte(',')
		prev = n
	}
	return b.String()
}

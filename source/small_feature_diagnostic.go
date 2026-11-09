package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Small-feature evidence is a READ-ONLY diagnostic. Mesh element IDs, a
// bounding-box proximity match and a CAD Face/Edge ID are different entities.
// No item in this structure is an approved sizing scope or a proven cause of
// poor skewness/element quality.
type smallFeatureFaceEvidence struct {
	FaceID         int
	EdgeID         int
	DistanceMM     float64
	ShortestEdgeMM float64
	AspectRatio    float64
	Method         string
	Hint           string
}

type smallFeatureCandidateView struct {
	CandidateID string
	BodyID      int
	FaceDetails []smallFeatureFaceEvidence
	Summary     string
	Advice      string
}

const maxFeatureDiagnosticCandidates = 256

// ACT and Geometry may have different units. An unknown unit means there can
// be no defensible spatial matching; do not silently assume mm.
func smallFeatureLengthFactor(unit string) (float64, bool) {
	key := strings.ToLower(strings.TrimSpace(unit))
	if dot := strings.LastIndex(key, "."); dot >= 0 {
		key = key[dot+1:]
	}
	switch key {
	case "standardnmm", "mm", "millimeter", "millimetre":
		return 1, true
	case "standardmks", "m", "meter", "metre":
		return 1000, true
	case "standardcgs", "cm", "centimeter":
		return 10, true
	case "standardumks", "um", "micrometer":
		return .001, true
	case "standardbin", "in", "inch":
		return 25.4, true
	case "standardbft", "ft", "feet":
		return 304.8, true
	}
	return 0, false
}

func featureFinite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }

type featureBox struct {
	lo, hi [3]float64
}

func (b featureBox) includes(point []float64) bool {
	if len(point) != 3 {
		return false
	}
	for axis := 0; axis < 3; axis++ {
		if !featureFinite(point[axis]) {
			return false
		}
		eps := 1.e-7 * math.Max(1, math.Max(math.Abs(b.lo[axis]), math.Abs(b.hi[axis])))
		if point[axis] < b.lo[axis]-eps || point[axis] > b.hi[axis]+eps {
			return false
		}
	}
	return true
}

// Mechanical ACT versions can serialize a six-number box as minXYZ/maxXYZ or
// minX/maxX/minY/maxY/minZ/maxZ. Only use the box if exactly one orientation
// is consistent with its own CAD centroid (or both describe the same box).
func verifiedFeatureBox(f Face, factor float64) (featureBox, bool) {
	if len(f.BBox) != 6 || len(f.Centroid) != 3 {
		return featureBox{}, false
	}
	var candidates []featureBox
	orders := [2][6]int{{0, 1, 2, 3, 4, 5}, {0, 2, 4, 1, 3, 5}}
	centroid := []float64{f.Centroid[0] * factor, f.Centroid[1] * factor, f.Centroid[2] * factor}
	for _, order := range orders {
		b := featureBox{}
		valid := true
		for j := 0; j < 3; j++ {
			b.lo[j] = f.BBox[order[j]] * factor
			b.hi[j] = f.BBox[order[j+3]] * factor
			if !featureFinite(b.lo[j]) || !featureFinite(b.hi[j]) || b.lo[j] > b.hi[j] {
				valid = false
			}
		}
		if valid && b.includes(centroid) {
			candidates = append(candidates, b)
		}
	}
	if len(candidates) == 0 {
		return featureBox{}, false
	}
	if len(candidates) == 2 {
		for i := 0; i < 3; i++ {
			if math.Abs(candidates[0].lo[i]-candidates[1].lo[i]) > 1.e-8 || math.Abs(candidates[0].hi[i]-candidates[1].hi[i]) > 1.e-8 {
				return featureBox{}, false // ambiguous ACT box ordering
			}
		}
	}
	return candidates[0], true
}

func featureBoxDistance(b featureBox, xyz []float64) float64 {
	if len(xyz) != 3 {
		return math.Inf(1)
	}
	d2 := 0.0
	for i := 0; i < 3; i++ {
		if !featureFinite(xyz[i]) {
			return math.Inf(1)
		}
		if xyz[i] < b.lo[i] {
			d2 += math.Pow(b.lo[i]-xyz[i], 2)
		} else if xyz[i] > b.hi[i] {
			d2 += math.Pow(xyz[i]-b.hi[i], 2)
		}
	}
	return math.Sqrt(d2)
}

// The bounding aspect ratio is only a hint for an elongated surface, not its
// wall thickness. A planar face legitimately has one zero bbox dimension.
func featureBoxAspect(b featureBox) float64 {
	dims := []float64{b.hi[0] - b.lo[0], b.hi[1] - b.lo[1], b.hi[2] - b.lo[2]}
	sort.Float64s(dims)
	if dims[1] < 1.e-7 || dims[2] <= 0 {
		return 0
	}
	return dims[2] / dims[1]
}

func diagnosticSmallFeatures(s *Snapshot, plan WarningPlan) []smallFeatureCandidateView {
	out := make([]smallFeatureCandidateView, 0, len(plan.Clusters))
	if s == nil {
		return out
	}
	factor, validUnits := smallFeatureLengthFactor(s.GeometryUnit)
	byBody := make(map[int][]Face)
	for _, face := range s.Faces {
		if face.ID <= 0 || face.Body <= 0 {
			continue
		}
		byBody[face.Body] = append(byBody[face.Body], face)
	}
	for i, c := range plan.Clusters {
		view := smallFeatureCandidateView{CandidateID: c.ID, BodyID: c.BodyID}
		switch {
		case i >= maxFeatureDiagnosticCandidates:
			view.Summary = "超过单次小特征报告分析上限；可缩小标记区间后重新诊断"
		case !validUnits:
			view.Summary = "CAD 长度单位未核验，不能进行面/边空间匹配"
		case len(c.CentreMM) != 3 || c.BodyID <= 0:
			view.Summary = "候选球心或 Body 身份不完整"
		default:
			view.FaceDetails = findSmallFeatureFaces(c, byBody[c.BodyID], factor)
			if len(view.FaceDetails) == 0 {
				view.Summary = "未取得可核验的附近 CAD Face/Edge；不能证明小凸起造成质量异常"
			} else {
				view.Summary = fmt.Sprintf("发现 %d 处待人工核验的附近 CAD 面/边线索；仅代表几何邻近，不代表缺陷成因", len(view.FaceDetails))
			}
		}
		view.Advice = "先核对真实高偏度/低 Element Quality 单元、短边或细长面及现有 Sizing；若确认 CAD Face，则人工创建 Face Named Selection，可用现有局部细化页安全审核。若只存在 CAD Edge，则在 Meshing 中手动检查 Edge Sizing；AutoMesh 当前不会自动发布 Edge Sizing。不得把网格单元 ID 当作 CAD 作用域。"
		out = append(out, view)
	}
	return out
}

func findSmallFeatureFaces(c WarningHotspot, faces []Face, factor float64) []smallFeatureFaceEvidence {
	matches := make([]smallFeatureFaceEvidence, 0, 8)
	manualMap := make(map[int]bool, len(c.MappedFaceIDs))
	for _, fid := range c.MappedFaceIDs {
		manualMap[fid] = true
	}
	// No global nearest-face assertion: the Body can span the entire reactor.
	// Only candidates with a reliable bbox near the ACT hotspot or a direct
	// Mesh.Element geometry-face link enter the report.
	maxDistance := math.Max(.4, math.Min(2.5, c.RadiusMM))
	for _, f := range faces {
		b, boxOK := verifiedFeatureBox(f, factor)
		_, direct := manualMap[f.ID]
		dist := math.Inf(1)
		aspect := 0.0
		if boxOK {
			dist = featureBoxDistance(b, c.CentreMM)
			aspect = featureBoxAspect(b)
		}
		if !direct && (!boxOK || dist > maxDistance) {
			continue
		}
		item := smallFeatureFaceEvidence{FaceID: f.ID, DistanceMM: dist, AspectRatio: aspect, Method: "同 Body CAD 面包围盒邻近（未证实对应）"}
		if direct {
			item.Method = "ACT 元素几何 Face ID 映射（仍需人工确认边/面关系）"
		}
		for _, e := range f.Edges {
			if e.ID > 0 && featureFinite(e.Length) && e.Length > 0 && (item.EdgeID == 0 || e.Length < item.ShortestEdgeMM/factor) {
				item.EdgeID = e.ID
				item.ShortestEdgeMM = e.Length * factor
			}
		}
		switch {
		case item.EdgeID != 0 && item.ShortestEdgeMM <= math.Max(1.5, 1.5*c.SizeMM):
			item.Hint = "检测到短 CAD 边，优先人工检查 Edge/Face Sizing 与几何尖角"
		case aspect >= 6:
			item.Hint = "面包围盒细长，建议核查窄面过渡及局部尺寸；并非实际壁厚"
		default:
			item.Hint = "仅空间邻近证据；核对实际单元位置后再决定是否细化"
		}
		matches = append(matches, item)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		ad := strings.HasPrefix(a.Method, "ACT 元素")
		bd := strings.HasPrefix(b.Method, "ACT 元素")
		if ad != bd {
			return ad
		}
		if a.Hint != b.Hint {
			as := strings.HasPrefix(a.Hint, "检测到短")
			bs := strings.HasPrefix(b.Hint, "检测到短")
			if as != bs {
				return as
			}
		}
		if a.DistanceMM != b.DistanceMM {
			return a.DistanceMM < b.DistanceMM
		}
		return a.FaceID < b.FaceID
	})
	if len(matches) > 4 {
		matches = matches[:4]
	}
	return matches
}

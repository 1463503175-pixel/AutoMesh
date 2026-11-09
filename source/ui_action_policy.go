package main

// A read-only task must not be blocked by Home's unrelated sizing reference.
// Only actions that actually consume the reference value validate that field.
func taskRequiresReferenceSize(phase string) bool {
	switch phase {
	case "prepare", "publish_names", "generate_reviewed", "auto", "auto_onepass":
		return true
	default:
		return false
	}
}

// Worksheet alternatives 0/1 do not use interval inputs; rules 2/3 do.
// Keep Element Quality in the same scan/identity gate as custom Skewness.
func isCustomQualityRule(rule int) bool { return rule == 2 || rule == 3 }

func markerRuleAction(rule int) string {
	switch rule {
	case 0:
		return "mark"
	case 1:
		return "markwarning"
	case 2:
		return "markhighskew"
	case 3:
		return "markloweq"
	default:
		return ""
	}
}

type workflowAvailability struct {
	Ready, Busy, Connected, Ack, Selected, Scanned, HasMarker, HasPreview, HasReport, HasNamed, CanGenerate, SkewRangeValid bool
}

// This controls button affordances; all real safety preconditions remain in the
// task dispatcher, including project fingerprint and review receipt checks.
func (s workflowAvailability) Allow(action string) bool {
	canConnect := s.Ready && !s.Busy
	canRead := canConnect && s.Connected && s.Ack && s.Selected
	canScan := canRead && s.Scanned
	switch action {
	case "auto":
		return canConnect
	case "link":
		return canConnect
	case "scan":
		return canRead
	case "qualitycheck", "analysisdeep", "diagnose", "mark", "markwarning":
		return canRead
	case "markhighskew", "markloweq":
		return canScan && s.SkewRangeValid
	case "warningplan":
		return canScan && s.HasMarker
	case "warningview":
		return s.HasReport
	case "warningpublish":
		return canScan && s.HasPreview
	case "finenames", "fineglobalreview", "finesuggest":
		// These are prerequisites/tools that must remain available when no
		// Named Selection has yet been created (first-time scans).
		return canScan
	case "fineprepare", "finechinese", "finebulkapply", "finegroupsave":
		return canScan && s.HasNamed
	case "finegenerate":
		return canScan && s.CanGenerate
	case "exportbutton":
		return canScan
	case "taskstatus", "taskstatusconn", "htmlreport", "report", "open", "tasklog":
		return true
	case "initretry":
		return !s.Busy
	case "clearlease":
		return !s.Busy
	case "storagechange":
		return !s.Busy
	default:
		return !s.Busy
	}
}

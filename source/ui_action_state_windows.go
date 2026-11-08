//go:build windows

package main

// Native EnableWindow disables pointer and keyboard activation, not just paint.
// The Workbench task dispatcher still performs all security/identity checks.
func (a *state) updateActionStates() {
	if a.window == 0 || a.ctl("navhome") == 0 {
		return
	}
	selection := int(send(a.ctl("systems"), CB_GETCURSEL, 0, 0))
	selected := selection >= 0 && selection < len(a.systems)
	ack := send(a.ctl("ack"), BM_GETCHECK, 0, 0) == 1
	scanned := a.snapshot != nil && selected && a.scannedSystem == a.systems[selection]
	marker := scanned && a.ws.hasCurrentMarker(a.snapshot)
	hasReport := a.ws.hasCurrentWarningReport(a.snapshot)
	rangeOK := validSkewRange(read(a.ctl("skewlow")), read(a.ctl("skewhigh")))
	reviewed := false
	if scanned && !a.busy {
		_, e := a.ws.ReadReviewedWarningPlan(a.snapshot)
		reviewed = e == nil
	}
	available := workflowAvailability{Ready: a.ready, Busy: a.busy, Connected: a.connected && a.port > 0 && len(a.systems) > 0, Ack: ack, Selected: selected,
		Scanned: scanned, HasMarker: marker, HasPreview: reviewed, HasReport: hasReport, HasNamed: scanned && len(a.selectedNamed) > 0, SkewRangeValid: rangeOK,
		CanGenerate: scanned && (a.preparedSystem == a.systems[selection] || hasVerifiedManagedManifest(a.ws.Dir, a.snapshot))}
	for _, name := range []string{"auto", "link", "scan", "qualitycheck", "diagnose", "mark", "markwarning", "markhighskew", "warningplan", "warningview", "warningpublish", "finenames", "fineprepare", "fineglobalreview", "finesuggest", "finechinese", "finebulkapply", "finegroupsave", "finegenerate", "exportbutton", "taskstatus", "analysisExport", "htmlreport", "report", "open", "tasklog", "initretry", "clearlease", "storagechange"} {
		enabled(a.ctl(name), available.Allow(name))
	}
	// Reading, selecting and editing group data while a background task is
	// active can result in stale settings. Let users still navigate/report.
	for _, name := range []string{"finegroups", "finegroupactive", "finegroupmm", "finebulkmm", "finefilter"} {
		enabled(a.ctl(name), available.Scanned && !a.busy)
	}
	// Freeze inputs while an ACT job is active: the Worker uses a submitted
	// snapshot, and editing visible inputs mid-run incorrectly implies those
	// edits alter the in-flight task. Native read-only navigation remains live.
	for _, name := range []string{"size", "global", "port", "export", "skewlow", "skewhigh", "ack", "systems"} {
		enabled(a.ctl(name), !a.busy)
	}
	if a.page == 6 {
		for _, key := range []string{"reviewlist", "reviewrisk", "reviewclear", "reviewback", "reviewdetails"} {
			enabled(a.ctl(key), !a.busy)
		}
		ids, budget := a.selectedCandidates()
		enabled(a.ctl("reviewconfirm"), !a.busy && a.reviewPublishBlockedReason == "" && len(ids) > 0 && budget <= 120000 && !a.reviewOverlapError && a.reviewDetailsOpened && send(a.ctl("reviewrisk"), BM_GETCHECK, 0, 0) == 1)
	}
	enabled(a.ctl("analysisExport"), len(a.qualityHistory) > 0 && !a.busy)
	if false {
	}
	refresh()
}

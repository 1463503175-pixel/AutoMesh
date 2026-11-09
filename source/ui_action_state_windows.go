//go:build windows

package main

// Native EnableWindow disables pointer and keyboard activation, not just paint.
// The Workbench task dispatcher still performs all security/identity checks.
func (a *state) updateActionStates() {
	if a.window == 0 || a.ctl("navhome") == 0 {
		return
	}
	view := a.sharedDataStatus()
	selection := int(send(a.ctl("systems"), CB_GETCURSEL, 0, 0))
	selected := selection >= 0 && selection < len(a.systems)
	ack := send(a.ctl("ack"), BM_GETCHECK, 0, 0) == 1
	scanned := view.GeometryState == SharedStateReady && selected
	marker := scanned && a.ws.hasCurrentMarker(a.snapshot)
	hasReport := a.ws.hasCurrentWarningReport(a.snapshot)
	rangeOK := validSkewRange(read(a.ctl("skewlow")), read(a.ctl("skewhigh")))
	reviewed := false
	if scanned && !a.busy {
		_, e := a.ws.ReadReviewedWarningPlan(a.snapshot)
		reviewed = e == nil
	}
	available := workflowAvailability{Ready: a.ready, Busy: a.busy, Connected: view.ConnectionState == SharedStateReady && a.port > 0 && len(a.systems) > 0, Ack: ack, Selected: selected,
		Scanned: scanned, HasMarker: marker, HasPreview: reviewed, HasReport: hasReport, HasNamed: scanned && len(a.selectedNamed) > 0, SkewRangeValid: rangeOK,
		CanGenerate: scanned && (a.preparedSystem == a.systems[selection] || hasVerifiedManagedManifest(a.ws.Dir, a.snapshot))}
	for _, name := range []string{"auto", "link", "scan", "qualitycheck", "analysisdeep", "diagnose", "mark", "markwarning", "markhighskew", "markexecute", "markrule", "warningplan", "warningview", "warningpublish", "finenames", "fineprepare", "fineglobalreview", "finesuggest", "finechinese", "finebulkapply", "finegroupsave", "finegenerate", "exportbutton", "taskstatus", "taskstatusconn", "htmlreport", "report", "open", "tasklog", "initretry", "clearlease", "storagechange"} {
		enabled(a.ctl(name), available.Allow(name))
	}
	markRule := int(send(a.ctl("markrule"), CB_GETCURSEL, 0, 0))
	markAction := markerRuleAction(markRule)
	markAllowed := markAction != "" && available.Allow(markAction)
	enabled(a.ctl("markexecute"), markAllowed)
	enabled(a.ctl("markrule"), available.Allow("mark"))
	// Users can fill the interval before a full scan; only task dispatch
	// requires the verified current geometry/Mesh identity.
	customRange := isCustomQualityRule(markRule) && !a.busy
	enabled(a.ctl("skewlow"), customRange)
	enabled(a.ctl("skewhigh"), customRange)
	enabled(a.ctl("qualityupdate"), available.Allow("qualitycheck"))
	enabled(a.ctl("syncstatus"), a.ready && !a.busy && a.connected && selected && ack)
	enabled(a.ctl("taskcenter"), true)
	enabled(a.ctl("taskdrawerclose"), a.taskCenterOpen)
	enabled(a.ctl("taskdrawerlog"), a.taskCenterOpen)
	enabled(a.ctl("analysisbase"), len(a.qualityHistory) > 0)
	enabled(a.ctl("analysistarget"), len(a.qualityHistory) > 0)
	enabled(a.ctl("analysisexport"), len(a.qualityHistory) > 0)
	enabled(a.ctl("analysisreport"), len(a.qualityHistory) > 0)
	enabled(a.ctl("actionhelp"), true)
	for _, name := range []string{"analysisoverview", "analysiscompare", "analysisdistribution", "analysisregion", "analysisrefresh"} {
		enabled(a.ctl(name), true)
	}
	// Reading, selecting and editing group data while a background task is
	// active can result in stale settings. Let users still navigate/report.
	for _, name := range []string{"finegroups", "finegroupactive", "finegroupmm", "finebulkmm", "finefilter"} {
		enabled(a.ctl(name), available.Scanned && !a.busy)
	}
	// A full geometry scan is the operation that makes the first Snapshot;
	// it must remain available before any scan exists.
	enabled(a.ctl("finerefresh"), available.Allow("scan"))
	// Freeze inputs while an ACT job is active: the Worker uses a submitted
	// snapshot, and editing visible inputs mid-run incorrectly implies those
	// edits alter the in-flight task. Native read-only navigation remains live.
	for _, name := range []string{"size", "global", "port", "export", "ack", "systems"} {
		enabled(a.ctl(name), !a.busy)
	}
	if a.page == 6 {
		for _, key := range []string{"reviewlist", "reviewrisk", "reviewclear", "reviewback", "reviewdetails"} {
			enabled(a.ctl(key), !a.busy)
		}
		ids, _ := a.selectedCandidates()
		enabled(a.ctl("reviewconfirm"), !a.busy && a.reviewPublishBlockedReason == "" && len(ids) > 0 && !a.reviewOverlapError && !a.reviewExistingNearDuplicate && a.reviewDetailsOpened && send(a.ctl("reviewrisk"), BM_GETCHECK, 0, 0) == 1)
	}
	refresh()
}

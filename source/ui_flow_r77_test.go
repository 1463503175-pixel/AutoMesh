package main

import "testing"

func TestQualityPageFollowsOperationalOrder(t *testing.T) {
	for _, width := range []int{1100, 1120, 1240, 1366, 1600, 1920} {
		c := plannedControls(width)
		// Inspection and exact-range selection must be seen before the candidate
		// preview and publish controls. Auxiliary diagnostics belong last.
		if c["qualitycheck"].Y >= c["warningplan"].Y || c["markhighskew"].Y >= c["warningplan"].Y {
			t.Errorf("%d: skew selection must precede preview", width)
		}
		if c["warningplan"].X >= c["warningview"].X || c["warningview"].X >= c["warningpublish"].X {
			t.Errorf("%d: preview/view/publish horizontal order incorrect", width)
		}
		if c["warningplan"].Y >= c["htmlreport"].Y {
			t.Errorf("%d: auxiliary report commands must be last", width)
		}
		if c["skewlow"].X >= c["skewhigh"].X || c["skewhigh"].X >= c["markhighskew"].X {
			t.Errorf("%d: lower/upper/select action order incorrect", width)
		}
		for _, k := range []string{"qualitycheck", "skewlow", "skewhigh", "markhighskew", "markwarning", "warningplan", "warningview", "warningpublish", "taskstatus", "htmlreport", "diagnose", "mark"} {
			b := c[k]
			if b.X+b.W > width-20 || b.Y+b.H > 690 {
				t.Errorf("%d: quality button %s clipped: %+v", width, k, b)
			}
		}
	}
}

func TestFineActionsAuditBeforeMeshGeneration(t *testing.T) {
	for _, width := range []int{1100, 1240, 1440, 1920} {
		c := plannedControls(width)
		if !(c["fineglobalreview"].X < c["finenames"].X && c["finenames"].X < c["fineprepare"].X && c["fineprepare"].X < c["finegenerate"].X) {
			t.Errorf("%d: global review must precede naming, sizing, generation", width)
		}
	}
}

func TestConnectionEmphasizesAutoConnectBeforeOptionalPort(t *testing.T) {
	for _, width := range []int{1100, 1240, 1600} {
		c := plannedControls(width)
		if c["link"].X >= c["port"].X {
			t.Errorf("%d: automatic connection should be first", width)
		}
	}
}

func TestHumanReadableTaskStates(t *testing.T) {
	checks := map[string]string{"warning_refine_preview": "生成局部优化方案", "warning_refine_publish": "发布局部 Sizing", "mark_high_skew": "标记偏度区间单元", "generate_reviewed": "生成网格"}
	for task, want := range checks {
		if got := taskDisplayName(task); got != want {
			t.Errorf("%s: got %q, expected %q", task, got, want)
		}
	}
	if taskStateText("ok") != "已完成" || taskStateText("error") != "失败" {
		t.Fatal("status is not localized")
	}
	if qualityGateText("critical_skewness") != "存在严重高偏度，需优化" {
		t.Fatal("high skewness warning lost")
	}
}

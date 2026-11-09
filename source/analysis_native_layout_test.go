package main

import "testing"

func TestAnalysisCenterNativeReportAndScrollableRegionsDoNotOverlap(t *testing.T) {
	for _, width := range []int{940, 1100, 1120, 1280, 1600, 1920} {
		controls := plannedControls(width)
		for _, a := range []string{"analysisoverview", "analysiscompare", "analysisdistribution", "analysisregion", "analysisreport"} {
			one := controls[a]
			if one.X+one.W > width && width >= 1100 {
				t.Errorf("%d: %s outside width", width, a)
			}
			for _, b := range []string{"analysisoverview", "analysiscompare", "analysisdistribution", "analysisregion", "analysisreport"} {
				if a != b && one.intersects(controls[b]) {
					t.Errorf("%d: %s and %s overlap", width, a, b)
				}
			}
		}
		for _, key := range []string{"analysisreport", "analysisbodylist"} {
			if p, ok := pageControlPage(key); !ok || p != 4 {
				t.Errorf("%s not assigned to Analysis Center", key)
			}
		}
		if y := controls["analysisbodylist"].Y + controls["analysisbodylist"].H; pageContentBottom(4) < y {
			t.Errorf("page scroll cannot reveal Body list ending at %d", y)
		}
		if controls["analysisbodylist"].intersects(controls["analysisbase"]) {
			t.Errorf("Body list intersects native selector")
		}
	}
}

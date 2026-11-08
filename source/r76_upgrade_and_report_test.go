package main

import "testing"

func TestR76R75AssetsCanUpgradeWithoutBlindOverwrite(t *testing.T) {
	if !knownLegacyWorkerHashes["a6d252a4d0a57c6b723b494280922c0cf83a0c3743e749bb4f6f4345037820d9"] {
		t.Fatal("the verified R7.5 worker was not whitelisted")
	}
	if !knownLegacyAssetHashes["warning_refinement.py"]["c13a583019f626528577642f00a7d15cc44ff5106604255d82affa811e9a2fc5"] {
		t.Fatal("the verified R7.5 clustering module was not whitelisted")
	}
}

func TestR80CandidatePlanDoesNotTruncateAt24(t *testing.T) {
	var ids []int
	for i := 1; i <= 13; i++ {
		ids = append(ids, i)
	}
	p := WarningPlan{SchemaVersion: 1, Status: "ready", PlanID: "reviewed", Project: "P", Fingerprint: "F", WarningCount: 13, CoveredCount: 13, Clusters: []WarningHotspot{}}
	for _, id := range ids {
		p.Clusters = append(p.Clusters, WarningHotspot{BodyID: 7225, CentreMM: []float64{float64(id), 0, 0}, RadiusMM: 2, SizeMM: 1, ElementIDs: []int{id}, BatchIndex: (id-1)/6 + 1})
	}
	s := &Snapshot{Project: "P", Fingerprint: "F"}
	if err := validateWarningDiagnostic(p, s, "reviewed"); err != nil {
		t.Fatal(err)
	}
	p.Clusters = append(p.Clusters, p.Clusters...)
	if err := validateWarningDiagnostic(p, s, "reviewed"); err != nil {
		t.Fatal("full inventory must include more than 24 candidates", err)
	}
}

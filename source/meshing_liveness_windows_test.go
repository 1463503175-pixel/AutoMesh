//go:build windows

package main

import "testing"

func TestMeshingProcessProbeRequestsSynchronize(t *testing.T) {
	if meshingProcessQueryAccess&processQueryLimitedInformation == 0 {
		t.Fatal("process probe must retain PROCESS_QUERY_LIMITED_INFORMATION")
	}
	if meshingProcessQueryAccess&processSynchronize == 0 {
		t.Fatal("WaitForSingleObject requires SYNCHRONIZE access")
	}
}

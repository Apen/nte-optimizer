package main

import "testing"

func TestAssembleUnrealMessagesCollectsReadablePayloadWithoutBunches(t *testing.T) {
	report := udpReport{}
	completed, inventoryGUIDs, dumpDir := assembleUnrealMessages(&report, []timedUDPPacket{{data: []byte("readable_payload")}}, "")
	if len(completed) != 0 || len(inventoryGUIDs) != 0 || dumpDir != "" {
		t.Fatalf("unexpected assembly result: completed=%d guids=%d dump=%q", len(completed), len(inventoryGUIDs), dumpDir)
	}
	if len(report.Readable) != 1 || report.Readable[0] != "readable_payload" {
		t.Fatalf("readable strings = %#v", report.Readable)
	}
}

func TestInspectCompletedMessagesHandlesEmptyInput(t *testing.T) {
	report := udpReport{}
	inspectCompletedMessages(&report, nil, nil, "")
	if report.InventoryPayloads != 0 || report.RPC256 != 0 || len(report.InventoryProperties) != 0 {
		t.Fatalf("empty inspection changed report: %#v", report)
	}
}

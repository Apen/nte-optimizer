package main

import (
	"reflect"
	"testing"
)

func TestDedupeUsesShortTimeWindow(t *testing.T) {
	packets := []packet{
		{ts: 0, data: []byte("same")},
		{ts: .001, data: []byte("same")},
		{ts: .003, data: []byte("same")},
		{ts: .0035, data: []byte("different")},
	}
	got := dedupe(packets)
	if len(got) != 3 || got[0].ts != 0 || got[1].ts != .003 || got[2].ts != .0035 {
		t.Fatalf("unexpected deduplicated packets: %#v", got)
	}
}

func TestTextInspectionHelpers(t *testing.T) {
	payload := []byte("noise\x00AchievementRecord\x00AlphaRecord\x00AlphaRecord\x00bad-value-Record\x00short\x00")
	if got := interestingMarkers(payload); !reflect.DeepEqual(got, []string{"AchievementRecord"}) {
		t.Fatalf("interestingMarkers() = %v", got)
	}
	if got := recordNames(payload); !reflect.DeepEqual(got, []string{"AchievementRecord", "AlphaRecord"}) {
		t.Fatalf("recordNames() = %v", got)
	}
	if got := asciiStrings([]byte("abc\x00hello world\x00ABCDEF"), 6); !reflect.DeepEqual(got, []string{"hello world", "ABCDEF"}) {
		t.Fatalf("asciiStrings() = %v", got)
	}
}

func TestPrivateIPv4Classification(t *testing.T) {
	tests := map[string]bool{
		"10.0.0.1": true, "172.16.0.1": true, "192.168.1.1": true,
		"127.0.0.1": true, "169.254.1.1": true, "224.0.0.1": true,
		"8.8.8.8": false, "invalid": true,
	}
	for address, want := range tests {
		if got := isPrivateIPv4(address); got != want {
			t.Errorf("isPrivateIPv4(%q) = %v, want %v", address, got, want)
		}
	}
}

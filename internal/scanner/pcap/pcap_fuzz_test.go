package pcap

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzParsePCAP(f *testing.F) {
	f.Add([]byte{0x0a, 0x0d, 0x0d, 0x0a})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		Parse(data)
	})
}

func TestOversizedCaptureRejectedBeforeReading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.pcapng")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxCaptureBytes + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path); err == nil {
		t.Fatal("accepted oversized capture")
	}
}

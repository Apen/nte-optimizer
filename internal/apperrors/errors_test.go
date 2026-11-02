package apperrors

import (
	"errors"
	"fmt"
	"testing"
)

func TestClientCodePreservesCauseWithoutExposingDetails(t *testing.T) {
	cause := errors.New("technical detail with a private location")
	err := fmt.Errorf("outer: %w", New("data_missing", cause))
	wire := ForClient(err)
	if wire.Error() != "nte:data_missing" || !errors.Is(wire, cause) {
		t.Fatalf("unsafe or untraceable client error: %v", wire)
	}
	if ForClient(nil) != nil {
		t.Fatal("nil error changed")
	}
}

package command

import "testing"

func TestGetMonitors(t *testing.T) {
	displays, err := getMonitors()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(displays) != 2 {
		t.Errorf("Expected 2 monitors, got %d", len(displays))
	}
}

func TestHandleDisplay(t *testing.T) {
	res := handleDisplay(Request{})

	if res.Type != "display" {
		t.Errorf("Expected display response, got %v (%v)", res.Type, res.Message)
	}
}

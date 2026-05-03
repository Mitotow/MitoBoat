package commands

import "testing"

func TestGetCommandFromMessage(t *testing.T) {
	if GetCommandFromMessage("") != nil {
		t.Errorf("An empty message cannot be a command")
	}

	msg := "test"
	if GetCommandFromMessage(msg) != nil {
		t.Errorf("Expected %s not to be a command", msg)
	}

	msg = "!"
	if GetCommandFromMessage(msg) != nil {
		t.Errorf("Expected %s not to be a command", msg)
	}

	msg = "!t"
	if GetCommandFromMessage(msg) == nil {
		t.Errorf("Expected %s to be a command", msg)
	}

	msg = "!test"
	if GetCommandFromMessage(msg) == nil {
		t.Errorf("Expected %s to be a command", msg)
	}

	msg = "!test test"
	if cmdName := GetCommandFromMessage(msg); cmdName == nil || *cmdName != "test" {
		t.Errorf("Expected %s to be a command named %s", msg, "test")
	}
}

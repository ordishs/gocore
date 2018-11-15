package gocore

import (
	"testing"
)

func TestGetExistingKey(t *testing.T) {
	name, ok := Config().Get("name")
	if name != "Simon" {
		t.Errorf("Expected 'Simon' and got '%s'", name)
	}

	if !ok {
		t.Errorf("Expected ok=true and got ok=%+v", ok)
	}
}

func TestGetExistingKeyInt(t *testing.T) {
	name, ok := Config().Get("tel")
	if name != "20289202982" {
		t.Errorf("Expected '20289202982' and got '%s'", name)
	}

	if ok != true {
		t.Errorf("Expected 'true' and got '%t'", ok)
	}

	if !ok {
		t.Errorf("Expected ok=true and got ok=%+v", ok)
	}
}

func TestGetNotExistingKey(t *testing.T) {
	name, ok := Config().Get("XXXXX")

	if name != "" {
		t.Errorf("Expected '' and got '%s'", name)
	}

	if ok {
		t.Errorf("Expected ok=false and got ok=%+v", ok)
	}
}

func TestGetNotExistingKeyWithDefault(t *testing.T) {
	name, ok := Config().Get("XXXXX", "FOUND")

	if name != "FOUND" {
		t.Errorf("Expected 'FOUND' and got '%s'", name)
	}

	if ok {
		t.Errorf("Expected ok=false and got ok=%+v", ok)
	}
}

func TestGetNotExistingKeyWithDefaultInt(t *testing.T) {
	name, ok := Config().GetInt("XXXXX", 72)

	if name != 72 {
		t.Errorf("Expected 72 and got '%d'", name)
	}

	if ok {
		t.Errorf("Expected ok=false and got ok=%+v", ok)
	}
}

func TestGetOutboundIP(t *testing.T) {
	ip, err := GetOutboundIP()
	if err != nil {
		t.Errorf("Expected IP, got %+v", err)
	}
	t.Logf("IP: %s", ip)
}

package utils

import (
	"fmt"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	val := "secret"

	c, err := encrypt(val)
	if err != nil {
		t.Error(err)
	}

	res, err := DecryptSetting(c)
	if err != nil {
		t.Error(err)
	}

	expected := fmt.Sprintf("*EHE*%s", val)
	if res != expected {
		t.Errorf("Expected %q, got %q", expected, res)
	}

	t.Logf("%s -> %s -> %s\n", val, c, res)
}

func TestEncryptSetting(t *testing.T) {
	val := "42"

	c, err := encrypt(val)
	if err != nil {
		t.Error(err)
	}

	t.Logf("%s -> %s\n", val, c)
}

func TestDecryptSetting(t *testing.T) {
	val := "*EHE*8f7d64a1f1cefb44fe280d40bfe056ebd3aff457dd551ab8edf5d213cf9c"

	res, err := DecryptSetting(val)
	if err != nil {
		t.Error(err)
	}

	expected := "*EHE*42"
	if res != expected {
		t.Errorf("Expected %q, got %q", expected, res)
	}

	t.Logf("%s -> %s\n", val, res)
}

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
	val := "*EHE*ad65473d70fc29d28823f7de3314bde3430d0b01b3cc2bc9556fdd343119265f400770683389a9ee15a8dd824f07f30a86000998dfee69ecf826436d55df"

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

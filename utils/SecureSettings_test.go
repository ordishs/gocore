package utils

import "testing"

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

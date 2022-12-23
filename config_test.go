package gocore

import (
	"path/filepath"
	"testing"

	"github.com/ordishs/gocore/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	ip, err := utils.GetOutboundIP()
	if err != nil {
		t.Errorf("Expected IP, got %+v", err)
	}
	t.Logf("IP: %s", ip)
}

func TestFilePath(t *testing.T) {
	file := "/Users/ordishs/dev/go/gocore/settings.conf"
	dir := filepath.Dir(file)
	if dir != "/Users/ordishs/dev/go/gocore" {
		t.Errorf("dir is wrong")
	}

	abs, _ := filepath.Abs(dir)
	if abs != "/Users/ordishs/dev/go/gocore" {
		t.Errorf("abs is wrong")
	}
}

func TestGetSecretKey(t *testing.T) {
	secret, ok := Config().Get("secret")
	if secret != "secret" {
		t.Errorf("Expected 'secret' and got '%s'", secret)
	}

	if !ok {
		t.Errorf("Expected ok=true and got ok=%+v", ok)
	}
}

func TestGetMagicNumber(t *testing.T) {
	secret, ok := Config().GetInt("magicNumber")
	if secret != 42 {
		t.Errorf("Expected 42 and got %d", secret)
	}

	if !ok {
		t.Errorf("Expected ok=true and got ok=%+v", ok)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	expected := "42"

	res, _ := Config().Get("magicNumber")
	if res != expected {
		t.Errorf("Expected %q, got %q", expected, res)
	}

	t.Logf("%s -> %s\n", expected, res)
}

func TestEncryptDecryptInt(t *testing.T) {
	expected := 42

	res, _ := Config().GetInt("magicNumber")
	if res != expected {
		t.Errorf("Expected %q, got %q", expected, res)
	}

	t.Logf("%d -> %d\n", expected, res)
}

func TestURL1(t *testing.T) {
	res, err, found := Config().GetURL("url1")
	require.NoError(t, err)

	password, set := res.User.Password()
	require.True(t, set)

	assert.Equalf(t, "http", res.Scheme, "scheme is wrong")
	assert.Equalf(t, "user", res.User.Username(), "username is wrong")
	assert.Equalf(t, "password", password, "password is wrong")
	assert.Equalf(t, "localhost", res.Hostname(), "hostname is wrong")
	assert.Equalf(t, "8080", res.Port(), "port is wrong")
	assert.Equalf(t, "", res.Path, "path is wrong")

	t.Logf("%v, %v", res, found)
}

func TestURLWithEncryptedPassword(t *testing.T) {
	res, err, found := Config().GetURL("url2")
	require.NoError(t, err)

	password, set := res.User.Password()
	require.True(t, set)

	assert.Equalf(t, "http", res.Scheme, "scheme is wrong")
	assert.Equalf(t, "user", res.User.Username(), "username is wrong")
	assert.Equalf(t, "password", password, "password is wrong")
	assert.Equalf(t, "localhost", res.Hostname(), "hostname is wrong")
	assert.Equalf(t, "8080", res.Port(), "port is wrong")
	assert.Equalf(t, "", res.Path, "path is wrong")

	t.Logf("%v, %v", res, found)
}

func TestURL3(t *testing.T) {
	res, err, found := Config().GetURL("url3")
	require.NoError(t, err)

	password, set := res.User.Password()
	require.False(t, set)

	assert.Equalf(t, "p2p", res.Scheme, "scheme is wrong")
	assert.Equalf(t, "", res.User.Username(), "username is wrong")
	assert.Equalf(t, "", password, "password is wrong")
	assert.Equalf(t, "localhost", res.Hostname(), "hostname is wrong")
	assert.Equalf(t, "8333", res.Port(), "port is wrong")
	assert.Equalf(t, "", res.Path, "path is wrong")

	t.Logf("%v, %v", res, found)
}

func TestURL4(t *testing.T) {
	res, err, found := Config().GetURL("url4")
	require.NoError(t, err)

	password, set := res.User.Password()
	require.False(t, set)

	assert.Equalf(t, "zmq", res.Scheme, "scheme is wrong")
	assert.Equalf(t, "", res.User.Username(), "username is wrong")
	assert.Equalf(t, "", password, "password is wrong")
	assert.Equalf(t, "localhost", res.Hostname(), "hostname is wrong")
	assert.Equalf(t, "28332", res.Port(), "port is wrong")
	assert.Equalf(t, "", res.Path, "path is wrong")

	t.Logf("%v, %v", res, found)
}

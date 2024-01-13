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
	assert.Equal(t, "Simon", name)
	assert.True(t, ok)
}

func TestGetMulti(t *testing.T) {
	res, ok := Config().GetMulti("multi", ",")
	require.True(t, ok)
	assert.Len(t, res, 3)
	assert.Equal(t, "simon", res[0])
	assert.Equal(t, "peter", res[1])
}

func TestGetEmbedded1(t *testing.T) {
	res, ok := Config().Get("city")
	require.True(t, ok)
	assert.Equal(t, "Paris", res)

	res, ok = Config().Get("embedded")
	require.True(t, ok)
	assert.Equal(t, "Simon lives in Paris", res)

	t.Log(res)
}

func TestGetExistingKeyInt(t *testing.T) {
	name, ok := Config().Get("tel")
	assert.Equal(t, "20289202982", name)
	assert.True(t, ok)
}

func TestGetNotExistingKey(t *testing.T) {
	name, ok := Config().Get("XXXXX")
	assert.Equal(t, "", name)
	assert.False(t, ok)
}

func TestGetNotExistingKeyWithDefault(t *testing.T) {
	name, ok := Config().Get("XXXXX", "FOUND")
	assert.Equal(t, "FOUND", name)
	assert.False(t, ok)
}

func TestGetNotExistingKeyWithDefaultInt(t *testing.T) {
	name, ok := Config().GetInt("XXXXX", 72)
	assert.Equal(t, 72, name)
	assert.False(t, ok)
}

func TestGetOutboundIP(t *testing.T) {
	_, err := utils.GetOutboundIP()
	assert.NoError(t, err)
	// assert.Equal(t, "172.20.40.202", ip.String())
}

func TestFilePath(t *testing.T) {
	file := "/Users/ordishs/dev/go/gocore/settings.conf"
	dir := filepath.Dir(file)
	assert.Equal(t, "/Users/ordishs/dev/go/gocore", dir)

	abs, _ := filepath.Abs(dir)
	assert.Equal(t, "/Users/ordishs/dev/go/gocore", abs)
}

func TestGetSecretKey(t *testing.T) {
	secret, ok := Config().Get("secret")
	assert.Equal(t, "secret", secret)
	assert.True(t, ok)
}

func TestGetMagicNumber(t *testing.T) {
	secret, ok := Config().GetInt("magicNumber")
	assert.Equal(t, 42, secret)
	assert.True(t, ok)
}

func TestEncryptDecrypt(t *testing.T) {
	res, _ := Config().Get("magicNumber")
	assert.Equal(t, "42", res)
}

func TestEncryptDecryptInt(t *testing.T) {
	res, _ := Config().GetInt("magicNumber")
	assert.Equal(t, 42, res)
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

// func TestGetContext(t *testing.T) {
// 	context := Config().GetContext()
// 	assert.Equalf(t, "dev", context, "context is wrong")
//}

func TestCheckDotEnv(t *testing.T) {
	val, _ := Config().Get("ENV_TEST")
	assert.Equal(t, "123", val)
}

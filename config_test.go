package gocore

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ordishs/gocore/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetExistingKey(t *testing.T) {
	name, ok := Config().Get("name")
	assert.Equal(t, "Simon", name)
	assert.True(t, ok)
}

func TestGetDuration(t *testing.T) {
	d, err, ok := Config().GetDuration("millis")
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, d)
	assert.True(t, ok)

	_, err, ok = Config().GetDuration("millisErr")
	require.Error(t, err)
	assert.Equal(t, `time: unknown unit "fg" in duration "2fg"`, err.Error())
	assert.False(t, ok)
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

func TestEmpty(t *testing.T) {
	val, found := Config().Get("empty")
	assert.True(t, found)
	assert.Equal(t, "", val)
}

func TestAlternativeContext(t *testing.T) {
	cSpecial := Config("special")
	assert.Equal(t, "special", cSpecial.context)

	v1, found := Config("special").Get("city")
	assert.True(t, found)
	assert.Equal(t, "Madrid", v1)
	c := Config()
	assert.Equal(t, os.Getenv("SETTINGS_CONTEXT"), c.context)

	v, found := c.Get("city")
	assert.True(t, found)
	assert.Equal(t, "Paris", v)
}

func TestDynamicVariables(t *testing.T) {
	v1, found := Config().Get("embedded")
	assert.True(t, found)
	assert.Equal(t, "Simon lives in Paris", v1)

	Config().Set("city", "London")

	v2, found := Config().Get("embedded")
	assert.True(t, found)
	assert.Equal(t, "Simon lives in London", v2)
}

func TestMissing(t *testing.T) {
	val, found := Config().Get("missing")
	assert.False(t, found)
	assert.Equal(t, "", val)
}

func TestEmptyEnvOverride(t *testing.T) {
	os.Setenv("city", "")
	city := os.Getenv("city")
	assert.Equal(t, "", city)

	val, found := Config().Get("city")
	assert.True(t, found)
	assert.Equal(t, "", val)
}

func TestGetUint(t *testing.T) {
	val, found := Config().GetUint("number")
	assert.True(t, found)
	assert.Equal(t, uint(5042), val)
}

func TestGetUint8(t *testing.T) {
	val, found := Config().GetUint8("number")
	assert.False(t, found)
	assert.Equal(t, uint8(0), val)

	val, found, err := Config().TryGetUint8("number")
	assert.Error(t, err)
	assert.True(t, found)
	assert.Equal(t, uint8(0), val)
}

// func TestApplication(t *testing.T) {
// 	val, found := Config().Get("app")
// 	assert.True(t, found)
// 	assert.Equal(t, "gocore", val)

// 	t.Log(Config().Stats())
// }

type mockListener struct {
	ch chan string
}

func newMockListener(bufferSize int) *mockListener {
	return &mockListener{
		ch: make(chan string, bufferSize),
	}
}

func (m *mockListener) UpdateSetting(key, value string) {
	select {
	case m.ch <- fmt.Sprintf("%s=%s", key, value):
	default:
		// Channel is full or closed, don't block
	}
}

func TestListener(t *testing.T) {
	t.Run("basic listener functionality", func(t *testing.T) {
		listener := newMockListener(1)
		Config().AddListener(listener)
		defer func() {
			Config().RemoveListener(listener)
			close(listener.ch)
		}()

		Config().Set("key1", "value1")

		select {
		case val := <-listener.ch:
			assert.Equal(t, "key1=value1", val)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for listener update")
		}
	})

	t.Run("multiple updates", func(t *testing.T) {
		listener := newMockListener(2)
		Config().AddListener(listener)
		defer func() {
			Config().RemoveListener(listener)
			close(listener.ch)
		}()

		Config().Set("key1", "value1")
		Config().Set("key2", "value2")

		expected := []string{"key1=value1", "key2=value2"}
		for i, want := range expected {
			select {
			case got := <-listener.ch:
				assert.Equal(t, want, got, "update %d", i+1)
			case <-time.After(time.Second):
				t.Fatalf("timeout waiting for update %d", i+1)
			}
		}
	})

	t.Run("removed listener receives no updates", func(t *testing.T) {
		listener := newMockListener(1)
		Config().AddListener(listener)
		Config().RemoveListener(listener)
		close(listener.ch)

		Config().Set("key", "value")

		select {
		case val, ok := <-listener.ch:
			if ok {
				t.Errorf("removed listener received update: %s", val)
			}
		default:
			// Expected - no updates should be received
		}
	})

	t.Run("full channel doesn't block", func(t *testing.T) {
		listener := newMockListener(1)
		Config().AddListener(listener)
		defer func() {
			Config().RemoveListener(listener)
			close(listener.ch)
		}()

		// Fill the channel
		Config().Set("key1", "value1")
		// This should not block even though channel is full
		Config().Set("key2", "value2")

		// Should receive at least the first update
		select {
		case val := <-listener.ch:
			assert.Equal(t, "key1=value1", val)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for listener update")
		}
	})
}

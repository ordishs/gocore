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

func TestEnvWithVariables(t *testing.T) {
	os.Setenv("city", "New York ${address}")
	city := os.Getenv("city")
	assert.Equal(t, "New York ${address}", city)

	val, found := Config().Get("city")
	assert.True(t, found)
	assert.Equal(t, "New York 1 The Main Street", val)

	os.Setenv("address", "2 The Main Street")

	val, found = Config().Get("city")
	assert.True(t, found)
	assert.Equal(t, "New York 2 The Main Street", val)

	os.Setenv("first", "Simon")
	os.Setenv("last", "Ordish")
	os.Setenv("fullname", "${first} ${last}")
	val, found = Config().Get("fullname")
	assert.True(t, found)
	assert.Equal(t, "Simon Ordish", val)
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

func TestInfo(t *testing.T) {
	SetInfo("test", "1.0.0", "1234567890")
	assert.Equal(t, "test", GetPackageName())
	assert.Equal(t, "1.0.0", GetVersion())
	assert.Equal(t, "1234567890", GetCommit())
}

func TestTestConfig(t *testing.T) {
	v, found := Config().Get("setting")
	assert.True(t, found)
	assert.Equal(t, "test", v)
}

func TestRequestRecordsDistinctByDefault(t *testing.T) {
	Config().Get("distinct_key", "ABC")
	Config().Get("distinct_key", "DEF")
	Config().Get("distinct_key")

	var count int
	for _, r := range Config().requestedSnapshot() {
		if r.Key == "distinct_key" {
			count++
		}
	}
	assert.Equal(t, 3, count)
}

func TestRequestSource(t *testing.T) {
	Config().Get("src_missing_key")

	os.Setenv("src_env_key", "hello")
	Config().Get("src_env_key")

	Config().Get("tel")

	src := func(key string) string {
		for _, r := range Config().requestedSnapshot() {
			if r.Key == key {
				return r.Source
			}
		}
		return ""
	}

	assert.Equal(t, "DEFAULT", src("src_missing_key"))
	assert.Equal(t, "ENV", src("src_env_key"))
	assert.Equal(t, "tel", src("tel"))
}

func TestRequestCountAndTimes(t *testing.T) {
	find := func() requestRecord {
		for _, r := range Config().requestedSnapshot() {
			if r.Key == "times_key" && r.HasDefault && r.DefaultValue == "x" {
				return r
			}
		}
		return requestRecord{}
	}

	Config().Get("times_key", "x")
	r1 := find()
	require.Equal(t, int64(1), r1.Count)

	time.Sleep(2 * time.Millisecond)

	Config().Get("times_key", "x")
	r2 := find()
	assert.Equal(t, int64(2), r2.Count)
	assert.Equal(t, r1.FirstRequested, r2.FirstRequested)
	assert.True(t, r2.LastRequested.After(r1.FirstRequested))
}

func TestRequestedMasksEHE(t *testing.T) {
	v, ok := Config().Get("secret")
	require.True(t, ok)
	assert.Equal(t, "secret", v)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "secret" {
			assert.Equal(t, eheMask, r.Value)
			return
		}
	}
	t.Fatal("secret was not recorded")
}

func TestRequestedMasksPunctuatedEHE(t *testing.T) {
	Config().Set("punct_secret", "*EHE*p@ssw0rd")

	_, _ = Config().Get("punct_secret")

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "punct_secret" {
			assert.Equal(t, eheMask, r.Value)
			return
		}
	}
	t.Fatal("punct_secret was not recorded")
}

func TestRequestedTextHeader(t *testing.T) {
	Config().Get("name")
	out := Config().Requested()
	assert.Contains(t, out, "KEY")
	assert.Contains(t, out, "SOURCE")
	assert.Contains(t, out, "COUNT")
	assert.Contains(t, out, "name")
}

func TestTypedGetterRecordsDefault(t *testing.T) {
	Config().GetInt("typed_missing", 99)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "typed_missing" {
			assert.True(t, r.HasDefault)
			assert.Equal(t, "99", r.DefaultValue)
			assert.Equal(t, "99", r.Value)
			assert.Equal(t, "DEFAULT", r.Source)
			return
		}
	}
	t.Fatal("typed_missing was not recorded")
}

func TestBoolGetterRecordsDefault(t *testing.T) {
	Config().GetBool("bool_missing", true)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "bool_missing" {
			assert.True(t, r.HasDefault)
			assert.Equal(t, "true", r.DefaultValue)
			assert.Equal(t, "DEFAULT", r.Source)
			return
		}
	}
	t.Fatal("bool_missing was not recorded")
}

func TestURLGetterRecordsSource(t *testing.T) {
	_, err, _ := Config().GetURL("url1")
	require.NoError(t, err)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "url1" {
			assert.Equal(t, "url1", r.Source)
			return
		}
	}
	t.Fatal("url1 was not recorded")
}

func TestDurationGetterRecordsFoundSource(t *testing.T) {
	_, err, ok := Config().GetDuration("millis")
	require.NoError(t, err)
	require.True(t, ok)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "millis" {
			assert.Equal(t, "millis", r.Source)
			assert.Equal(t, "2s", r.Value)
			return
		}
	}
	t.Fatal("millis was not recorded")
}

func TestReplaceVariablesNoPollution(t *testing.T) {
	Config().Set("polvar", "hello")
	Config().Set("uses_polvar", "prefix ${polvar}")

	val, ok := Config().Get("uses_polvar")
	require.True(t, ok)
	assert.Equal(t, "prefix hello", val)

	for _, r := range Config().requestedSnapshot() {
		if r.Key == "polvar" {
			t.Fatal("interpolation-only var 'polvar' must not be recorded as requested")
		}
	}
}

func TestStatsFormatUnchanged(t *testing.T) {
	s := Config().Stats()

	assert.Contains(t, s, "\nCMDLINE\n-------\n")
	assert.Contains(t, s, "\nSETTINGS\n--------\n")
	assert.Contains(t, s, "name=Simon\n")
	assert.Contains(t, s, "tel=20289202982\n")
	assert.Contains(t, s, "secret="+eheMask+"\n")
	assert.Contains(t, s, "magicNumber="+eheMask+"\n")
}

func TestRequestCountByKey(t *testing.T) {
	Config().Get("reqcount_key")
	Config().Get("reqcount_key")

	counts := Config().requestCountByKey()
	assert.GreaterOrEqual(t, counts["reqcount_key"], int64(2))
	_, present := counts["reqcount_never_requested_key"]
	assert.False(t, present)
}

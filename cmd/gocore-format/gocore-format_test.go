package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanMultiValues(t *testing.T) {
	v := "1|2|3"

	cleaned := cleanMultiValues(v)

	assert.Equal(t, "1 | 2 | 3", cleaned)
}

func TestEmptyValues(t *testing.T) {
	reader := strings.NewReader(`
		a=
		a.dev= #Comment
	`)

	settings, err := readSettings(reader)
	require.NoError(t, err)

	sortSettings(settings)

	// Write settings to a string buffer
	buf := &bytes.Buffer{}
	err = writeSettings(buf, settings)
	require.NoError(t, err)

	assert.Equal(t, `a     =
a.dev = # Comment
`, buf.String())
}

func TestComments(t *testing.T) {
	reader := strings.NewReader(`
		a=2
		A=1
		#The following section is c
		c=3 #this is the default value
		c.dev=1
		#c.test=2
		c.prod=3
	`)

	settings, err := readSettings(reader)
	require.NoError(t, err)

	sortSettings(settings)

	// Write settings to a string buffer
	buf := &bytes.Buffer{}
	err = writeSettings(buf, settings)
	require.NoError(t, err)

	assert.Equal(t, 3, len(settings))
	assert.Equal(t, `A = 1

a = 2

# The following section is c
c        = 3 # this is the default value
c.dev    = 1
# c.test = 2
c.prod   = 3
`, buf.String())
}

func TestGroups(t *testing.T) {
	reader := strings.NewReader(`
		a=2
		# @group: S1 compact
		A=1
		C=3
		B=2

		E=5
		D=4
		# @endgroup

		#The following section is c
		# @group: c
		c=0 #this is the default value
		c.dev=1
		#c.test=2 # This is not used at the moment
		c.prod=3
		something.else.c = 19
		# @endgroup

		b.c = 10
		b.d = 11
	`)

	settings, err := readSettings(reader)
	require.NoError(t, err)

	sortSettings(settings)

	// Write settings to a string buffer
	buf := &bytes.Buffer{}
	err = writeSettings(buf, settings)
	require.NoError(t, err)

	assert.Equal(t, 9, len(settings))
	assert.Equal(t, `# @group: S1 compact
A = 1
B = 2
C = 3
D = 4
E = 5
# @endgroup

a = 2

b.c = 10
b.d = 11

# @group: c
# The following section is c
c        = 0 # this is the default value
c.dev    = 1
# c.test = 2 # This is not used at the moment
c.prod   = 3

something.else.c = 19
# @endgroup
`, buf.String())
}

func TestProcessLine(t *testing.T) {
	test := []struct {
		line string
		want *Variant
	}{
		{
			line: "#a=b",
			want: &Variant{
				Commented: true,
				Key:       "a",
				Value:     "b",
			},
		},
		{
			line: "a=b #comment",
			want: &Variant{
				Commented: false,
				Key:       "a",
				Value:     "b",
				Comment:   "comment",
			},
		},
		{
			line: "#comment",
			want: nil,
		},
	}

	for _, tt := range test {
		t.Run(tt.line, func(t *testing.T) {
			setting := processLine(tt.line)
			assert.Equal(t, tt.want, setting)
		})
	}
}

package gocore

import (
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type BufferWithClose struct {
	*bytes.Buffer
	mu sync.Mutex
}

func (b *BufferWithClose) Close() error {
	return nil
}

func (b *BufferWithClose) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}

func (b *BufferWithClose) Read(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Read(p)
}

func TestHandleConfig(t *testing.T) {
	buf := &BufferWithClose{Buffer: &bytes.Buffer{}}

	socketHandler := NewSocketHandler(Log("test"), buf)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "set bob=simon says",
			args: []string{"", "set", "bob=simon says more"},
			want: "  Created new setting: bob=simon says more\n\n",
		},
		{
			name: "set bob2 simon says",
			args: []string{"", "set", "bob2", "simon says"},
			want: "  Created new setting: bob2=simon says\n\n",
		},
		{
			name: "set bob2 simon says again",
			args: []string{"", "set", "bob2", "simon says again"},
			want: "  Updated setting: bob2 simon says -> simon says again\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wg sync.WaitGroup
			wg.Add(1)

			var result string
			go func() {
				defer wg.Done()
				b := make([]byte, 1024)
				n, err := buf.Read(b)
				if err != nil && err != io.EOF {
					t.Errorf("unexpected error reading buffer: %v", err)
					return
				}
				result = string(b[:n])
			}()

			socketHandler.handleConfig(tt.args)
			wg.Wait()

			assert.Equal(t, tt.want, result, tt.name)
		})
	}
}

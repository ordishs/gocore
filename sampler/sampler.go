package sampler

import (
	"fmt"
	"os"
	"path/filepath"
)

// Sampler struct
type Sampler struct {
	ID       string
	Filename string
	Regex    string
	ch       chan string
	f        *os.File
}

// New creates a new sampler
func New(id string, filename string, regex string) (*Sampler, error) {
	ch := make(chan string)

	f, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	go func() {
		defer f.Close()

		for msg := range ch {
			f.Write([]byte(msg))
		}
	}()

	return &Sampler{
		ID:       id,
		Filename: filename,
		Regex:    regex,
		ch:       ch,
		f:        f,
	}, nil

}

func (s *Sampler) Write(str string) {
	defer func() {
		// recover from panic caused by writing to a closed channel
		if r := recover(); r != nil {
			err := fmt.Errorf("%v", r)
			fmt.Printf("write: error writing %s on channel: %v\n", str, err)
			return
		}
	}()

	s.ch <- str
}

// Stop the sampler
func (s *Sampler) Stop() {
	// Closing the channel should get the go routine to end and call the defer s.f.Close()
	close(s.ch)
}

func (s *Sampler) String() string {
	abs, err := filepath.Abs(s.f.Name())
	if err != nil {
		abs = s.f.Name()
	}

	if s.Regex == "" {
		return fmt.Sprintf("Sampler %s: writing all logs to %s", s.ID, abs)
	}

	return fmt.Sprintf("Sampler %s: writing logs that match %q to %s", s.ID, s.Regex, abs)
}

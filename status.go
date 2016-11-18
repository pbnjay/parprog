package parprog

import (
	"fmt"
	"os"
	"time"
)

const Wheel = "/-\\|"

type readStatusInterface interface {
	readStatus() string
	done()
}

///////////////////

// spinner spins a wheel each time status is updated...
type spinner struct {
	w       int
	start   time.Time
	elapsed time.Duration
}

func newSpinner() *spinner {
	return &spinner{
		start: time.Now().Truncate(time.Second),
	}
}

func (s *spinner) readStatus() string {
	if s.elapsed != 0 {
		return fmt.Sprintf("%s %6.2f%%", s.elapsed.String(), 100.0)
	}
	s.w = (s.w + 1) % len(Wheel)
	return fmt.Sprintf("%s    %c   ",
		time.Now().Truncate(time.Second).Sub(s.start).String(),
		Wheel[s.w])
}

func (s *spinner) done() {
	s.elapsed = time.Now().Truncate(time.Second).Sub(s.start)
	if s.elapsed == 0 {
		s.elapsed = time.Second
	}
}

//////////

// fileWrapper shows percent completion by comparing current file offset to size
type fileWrapper struct {
	sz      float64
	f       *os.File
	start   time.Time
	elapsed time.Duration
}

func (w *fileWrapper) done() {
	w.elapsed = time.Now().Truncate(time.Second).Sub(w.start)
}

func wrapFile(f *os.File) (*fileWrapper, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return &fileWrapper{
		sz:    float64(info.Size()) / 100.0,
		f:     f,
		start: time.Now().Truncate(time.Second),
	}, nil
}

func (w *fileWrapper) readStatus() string {
	if w.elapsed != 0 {
		return fmt.Sprintf("%s %6.2f%%", w.elapsed.String(), 100.0)
	}
	pos, err := w.f.Seek(0, os.SEEK_CUR)
	if err != nil {
		w.elapsed = time.Now().Truncate(time.Second).Sub(w.start)
		return fmt.Sprintf("%s %6.2f%%", w.elapsed.String(), 100.0)
	}
	return fmt.Sprintf("%s %6.2f%%",
		time.Now().Truncate(time.Second).Sub(w.start).String(),
		float64(pos)/w.sz)
}

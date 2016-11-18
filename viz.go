// Package parprog implements a parallel progress display so that one can read
// and process multiple files in separate gorouties with a visual progress
// indicator in the terminal. In addition to the Viz methods, a helper func
// BoundedExec will allow calling code to easily limit concurrent readers.
//
//    v := &parprog.Viz{}
//    v.Start(time.Second)
//    parprog.BoundedExec(3, flag.Args(), func(fn string) {
//      basename := filepath.Base(fn)
//      f, err := os.Open(fn)
//      v.Add(basename, f) // NB added even if invalid
//      if err != nil {
//        // Viz controls the terminal, so this is only way to display errors
//        v.Complete(basename, err)
//        return
//      }
//      defer v.Complete(basename, nil)
//
//      // ... long-running code here ...
//
//    })
//    v.Stop()
//
package parprog

import (
	"compress/gzip"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/nsf/termbox-go"
)

//////////

type readInfo struct {
	Name  string
	View  readStatusInterface
	Error error
}

// Viz provides a wrapper for multiple progress / status displays for parallel
// readers in process. The zero value struct is ready to be Start()-ed.
type Viz struct {
	mu      *sync.Mutex
	readers []readInfo

	interval time.Duration
	quit     chan int
	started  time.Time
}

// Start sets up the terminal for displaying reader progress, refreshed at the
// given interval in a background goroutine. After calling Start, Stop() must
// be called to stop the goroutine and return the terminal to a sane state.
func (v *Viz) Start(refreshInterval time.Duration) {
	termbox.Init()
	termbox.HideCursor()
	v.mu = &sync.Mutex{}
	v.started = time.Now().Truncate(time.Second)
	v.interval = refreshInterval
	v.quit = make(chan int)
	go func() {
		for {
			ev := termbox.PollEvent()
			if ev.Type == termbox.EventInterrupt {
				// termbox is closing
				return
			}
			if ev.Key == termbox.KeyCtrlC {
				v.quit <- 1
				return
			}
		}
	}()

	go v.run()
}

func (v *Viz) run() {
	ticker := time.NewTicker(v.interval)
	for {
		select {
		case q := <-v.quit:
			ticker.Stop()
			if q == 0 {
				termbox.Interrupt()
			}
			termbox.Close()
			if q != 0 {
				os.Exit(q)
			}
			return
		case <-ticker.C:
			v.mu.Lock()
			v.redrawLocked()
			v.mu.Unlock()
		}
	}
}

func (v *Viz) redrawLocked() {
	w, h := termbox.Size()
	termbox.Clear(termbox.ColorBlack, termbox.ColorDefault)
	ri := len(v.readers)
	ymax := len(v.readers) + 1
	if ymax > h {
		ymax = h
	}
	s := time.Now().Truncate(time.Second).Sub(v.started).String()
	s = "Running for " + s
	for i, c := range s {
		if i >= w {
			break
		}
		termbox.SetCell(i, 0, c, termbox.ColorWhite, termbox.ColorDefault)
	}

	for y := 1; y < ymax; y++ {
		ri--
		r := v.readers[ri]
		es := ""
		if r.Error != nil {
			es = r.Error.Error()
		}
		s := fmt.Sprintf("%15s %s %s", r.View.readStatus(), r.Name, es)

		fg := termbox.ColorWhite
		for i, c := range s {
			if i >= w {
				break
			}
			if i == 15 {
				fg = termbox.ColorDefault
			} else if i == len(s)-len(es) {
				fg = termbox.ColorRed | termbox.AttrBold
			}
			termbox.SetCell(i, y, c, fg, termbox.ColorDefault)
		}
	}
	termbox.Flush()
}

// Stop kills the display goroutine and cleans up the terminal display.
func (v *Viz) Stop() {
	v.quit <- 0
	time.Sleep(v.interval)
}

// Add a reader to the Viz. An *os.File will give best results showing percent
// completion using Seek and Stat calls to compute offsets and file size.
// Otherwise, a spinner will be displayed along with the name and time elapsed.
func (v *Viz) Add(name string, rdr interface{}) {
	info := readInfo{
		Name: name,
	}

	switch x := rdr.(type) {
	case *gzip.Reader:
		info.Error = fmt.Errorf("cannot inspect gzip.Reader, use wrapped Reader")
		info.View = newSpinner()

	case *os.File:
		info.View, info.Error = wrapFile(x)
		if info.Error != nil {
			info.View = newSpinner()
		}
	default:
		info.View = newSpinner()
	}

	v.mu.Lock()
	v.readers = append(v.readers, info)
	v.redrawLocked()
	v.mu.Unlock()
}

// Complete marks a reader as completed in the Viz by name. If an error is
// provided, it will be added to the display.
func (v *Viz) Complete(name string, err error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for i, x := range v.readers {
		if x.Name == name {
			x.View.done()
			x.Error = err
			v.readers[i] = x
			return
		}
	}
}

// Remove a reader from the Viz by name.
func (v *Viz) Remove(name string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for i, x := range v.readers {
		if x.Name == name {
			if i == len(v.readers)-1 {
				v.readers = v.readers[:i-1]
				return
			}

			rr := v.readers[i+1:]
			v.readers = v.readers[:i]
			v.readers = append(v.readers, rr...)
			return
		}
	}
}

package parprog

import "sync"

// BoundedExec provides a way to limit the number of concurrent goroutines (for
// example when doing parallel reads when I/O contention is more of an issue
// than CPU contention).
//
// At most n nameFunc()s will be called in parallel on every member of names.
func BoundedExec(n int, names []string, nameFunc func(string)) {
	wg := sync.WaitGroup{}
	boundedChan := make(chan string, n)

	for i := 0; i < n; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			for {
				name, ok := <-boundedChan
				if !ok {
					return
				}
				nameFunc(name)
			}
		}()
	}

	for _, fn := range names {
		boundedChan <- fn
	}
	close(boundedChan)

	wg.Wait()
}

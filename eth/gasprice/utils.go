package gasprice

import "sync"

// CirculeQueue currently is only for special usage.
// Thread unsafe!
type CirculeQueue struct {
	items []interface{}
	cap   int //
	n     int // lenght
	i     int // start index
	e     int // end index
}

func NewCirculeQueue(c int) *CirculeQueue {
	if c <= 0 {
		panic("capacity must greater than 0")
	}
	return &CirculeQueue{
		items: make([]interface{}, 0, c),
		cap:   c,
	}
}

func NewCirculeQueueByItems(items []interface{}) *CirculeQueue {
	its := make([]interface{}, len(items))
	copy(its, items)
	return &CirculeQueue{
		items: its,
		cap:   len(its),
		n:     len(its),
		i:     len(its) - 1,
	}
}

// EnAndReplace enqueue one price and return the replaced one,
// if there's no item replaced, the return will be nil.
func (q *CirculeQueue) EnAndReplace(b interface{}) (d interface{}) {
	if q.n == q.cap {
		d = q.items[q.e]
		q.i = q.e
		q.items[q.i] = b
		q.e = (q.e + 1) % q.cap
	} else {
		q.i = (q.i + 1) % q.cap
		q.items[q.i] = b
		q.n++
	}
	return
}

// Stats statistics tx count of the last few blocks
type Stats struct {
	q   *CirculeQueue
	n   int
	sum int
	avg int

	lock sync.RWMutex
}

func NewStats(txc []int) *Stats {
	n := len(txc)
	its := make([]interface{}, n)
	total := 0
	for i, v := range txc {
		its[i] = v
		total += v
	}
	q := NewCirculeQueueByItems(its)
	return &Stats{
		q:   q,
		n:   n,
		sum: total,
		avg: total / n,
	}
}

func (s *Stats) Add(cnt int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	d := s.q.EnAndReplace(cnt)
	i := d.(int)
	s.sum += cnt - i
	s.avg = s.sum / s.n
}

func (s *Stats) Avg() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.avg
}

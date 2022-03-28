package counter

import "github.com/cheggaaa/pb"

func New(total int) *Counter {
	return &Counter{
		c:     pb.StartNew(total),
		total: total,
	}
}

type Counter struct {
	c       *pb.ProgressBar
	current int
	total   int
}

func (c *Counter) Increment() {
	c.current++
	c.c.Increment()
}

func (c *Counter) Finish() {
	if c.current < c.total {
		c.c.SetTotal(c.current)
	}

	c.c.Finish()
}

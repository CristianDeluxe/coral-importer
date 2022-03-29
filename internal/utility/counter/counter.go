package counter

import (
	"github.com/cheggaaa/pb/v3"
	"github.com/fatih/color"
)

func New(title string, total int) *Counter {
	color.New(color.Bold).Printf("\n%s\n", title)

	return &Counter{
		c:     pb.Full.Start(total),
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
		c.c.AddTotal(int64(c.current - c.total))
	}

	c.c.Finish()
}

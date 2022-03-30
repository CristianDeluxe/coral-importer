package counter

import (
	"github.com/cheggaaa/pb/v3"
	"github.com/fatih/color"
)

func New(title string, total int) *Counter {
	color.New(color.Bold).Printf("\n%s\n", title)

	return &Counter{
		bar: pb.Full.Start(total),
	}
}

type Counter struct {
	bar *pb.ProgressBar
}

func (c *Counter) Increment() {
	c.bar.Increment()
}

func (c *Counter) Finish() {
	if c.bar.Current() < c.bar.Total() {
		c.bar.AddTotal(c.bar.Current() - c.bar.Total())
	}

	c.bar.Finish()
}

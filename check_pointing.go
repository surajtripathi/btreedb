package btreestore

type CheckPointing struct {
	threshold int
	counter   int
}

func newCheckPointing(threshold int) *CheckPointing {
	return &CheckPointing{
		threshold: threshold,
		counter:   0,
	}
}

func (c *CheckPointing) reset() {
	c.counter = 0
}
func (c *CheckPointing) check() bool {
	return c.counter >= c.threshold
}
func (c *CheckPointing) incr() {
	c.counter++
}

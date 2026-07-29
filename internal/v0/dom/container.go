package dom

func (c *Container) NodePointers() NodePointers { return c.nodePointers }
func (c *Container) LayoutBox() LayoutBox       { return c.layoutBox }
func (c *Container) Style() Style               { return c.style }

func (c *Container) Layout(ctx *LayoutContext, parentBox LayoutBox) {
	w := c.style.Width
	if w == 0 {
		// subtract the padding
		w = parentBox.Width - (c.style.Padding.Left + c.style.Padding.Right)
	}
}

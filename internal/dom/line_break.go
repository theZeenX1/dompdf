package dom

func (p *ParagraphNode) LineBreak(ctx LayoutContext) {
	parent, ok := ctx.ElementCache[p.nodePointers.parentNodeId]
	if !ok {
		// paragraph node is the root
	} else {
		// paragraph has a parent node
		(*parent).LayoutBox()
	}
	for _, frag := range p.Fragments {
		frag.inline()
	}
}

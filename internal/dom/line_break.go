package dom

func (p *ParagraphNode) LineBreak() {
	for _, inode := range p.Inlines {
		switch inode.(type) {
		case TextNode:
		}
	}
}

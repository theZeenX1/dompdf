package dom

type FlexDirection int16

const (
	FlexRow FlexDirection = iota
	FlexRowReverse
	FlexColumn
	FlexColumnReverse
)

type Align int16

const (
	AlignStart Align = iota
	AlignEnd
	AlignCenter
	AlignStretch
	AlignBaseline
)

type Justify int16

const (
	JustifyStart Justify = iota
	JustifyEnd
	JustifyCenter
	JustifySpaceBetween
	JustifySpaceAround
	JustifySpaceEvenly
)

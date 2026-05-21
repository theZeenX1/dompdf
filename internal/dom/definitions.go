package dom

type MeasurementType int16

const (
	Pt MeasurementType = iota
	Percentage
)

type BorderStyle int16

const (
	Solid BorderStyle = iota
	Dashed
	Dotted
	Double
)

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

type TextAlign int16

const (
	TextAlignStart TextAlign = iota
	TextAlignEnd
	TextAlignCenter
)

type FloatLayout int16

const (
	FloatCenter FloatLayout = iota
	FloatStart
	FloatEnd
)

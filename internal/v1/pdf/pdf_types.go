package pdf

type PositionMatrix struct {
	a, b, c, d, x, y float64
}

type Rectangle struct {
	LLX, LLY, URX, URY float64
}

type DocumentInfo struct {
	Title    string
	Author   string
	Subject  string
	Creator  string
	Producer string
}

package types

import (
	"mime/multipart"
	"regexp"
)

type RasterKeySettings struct {
	Type    string
	NumChar int
	Regex   *regexp.Regexp
}
type RasterFeatureSettings struct {
	XPosition string
	YPosition string
	Margin    float64
}
type GeoreferenceSettings struct {
	MasterMap             string
	AttrKey               string
	RasterKeySettings     *RasterKeySettings
	TargetDir             string
	SeparateDirAttrs      []string
	RasterFeatureSettings *RasterFeatureSettings
}
type GeoreferenceRequest struct {
	Raster   []*multipart.FileHeader
	Settings *GeoreferenceSettings
}

type MasterMap struct {
	Name      string `json:"name"`
	Dimension int    `json:"dimension"`
	Srid      int    `json:"srid"`
	Category  string `json:"type"`
}
type MasterMapAttr struct {
	Name     string `json:"name"`
	Category string `json:"type"`
}
type Dimension struct {
	Length, Width float64
}
type Diagonal struct {
	TopLeft, TopRight, BottomLeft, BottomRight Coord
}
type Margin struct {
	MarginX, MarginY float64
}
type Coord []float64

type FeaturePoints struct {
	Points []Coord `json:"points"`
}

type Extent struct {
	MinX float64 `json:"minX"`
	MinY float64 `json:"minY"`
	MaxX float64 `json:"maxX"`
	MaxY float64 `json:"maxY"`
}

type WorldFileParameter struct {
	A, D, B, E, C, F float64
}
type Result struct {
	Id    string
	Error error
}

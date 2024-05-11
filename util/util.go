package util

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"math"
	"mime/multipart"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/nahargo/geomatis-api/types"
	"github.com/rwcarlsen/goexif/exif"
)

func init() {
	// damn important or else At(), Bounds() functions will
	// caused memory pointer error!!
	image.RegisterFormat("jpeg", "jpeg", jpeg.Decode, jpeg.DecodeConfig)
	image.RegisterFormat("png", "png", png.Decode, png.DecodeConfig)
}
func LW(a types.Dimension) types.Dimension { //Correct Length Width Dimension
	d := a
	if a.Width > a.Length {
		d.Length = a.Width
		d.Width = a.Length
	}
	return d
}
func GetFeatureDimensions(d types.Diagonal, margin float64) (types.Dimension, types.Dimension) {

	w := math.Pow(math.Pow(d.TopLeft[0]-d.TopRight[0], 2)+math.Pow(d.TopLeft[1]-d.TopRight[1], 2), 0.5)     // pythagoras formula
	h := math.Pow(math.Pow(d.TopLeft[0]-d.BottomLeft[0], 2)+math.Pow(d.TopLeft[1]-d.BottomLeft[1], 2), 0.5) // pythagoras formula
	featureImg := types.Dimension{
		Length: (1 / (1 + margin)) * w,
		Width:  (1 / (1 + margin)) * h,
	}
	marginAroundFeature := types.Dimension{
		Length: (w - featureImg.Length) / 2,
		Width:  (h - featureImg.Width) / 2,
	}

	return featureImg, marginAroundFeature
}
func DimRatio(dim types.Dimension) float64 {
	return dim.Length / dim.Width
}
func calculateCentroid(points []types.Coord) types.Coord {
	var sumX, sumY float64

	// Calculate the sum of all x and y coordinates
	for _, p := range points {
		sumX += p[0]
		sumY += p[1]
	}

	// Calculate the centroid by averaging the x and y coordinates
	centroidX := sumX / float64(len(points))
	centroidY := sumY / float64(len(points))

	return types.Coord{centroidX, centroidY}
}
func CalculateGeoreferenceParameters(img types.Dimension, rasterPoints []types.Coord, extent types.Extent, margin float64) *types.WorldFileParameter {
	// WorldFileParameter : A, D, B, E, C, F
	var scale, x, y float64

	deltaX := extent.MaxX - extent.MinX
	deltaY := extent.MaxY - extent.MinY
	var polygon types.Dimension
	polygon.Length, polygon.Width = deltaX, deltaY

	diagonal, _ := FindDiagonalPoints(rasterPoints)
	featureImg, _ := GetFeatureDimensions(diagonal, margin)

	angle := CalculateRotationAngle(diagonal.TopLeft, diagonal.TopRight)
	if deltaX > deltaY || img.Length > img.Width {
		if deltaX < deltaY { // portrait polygon
			angle = angle - 90
		} else if img.Length < img.Width {
			angle = angle + 90
		}
	}

	LWPolygon := LW(polygon)
	LWFeatureImg := LW(featureImg)

	radian := math.Pi * angle / 180.0
	if DimRatio(LWPolygon) >= DimRatio(LWFeatureImg) { // condong horizontal
		//Scale X
		scale = LWPolygon.Length / LWFeatureImg.Length

	} else {
		//Scale Y
		scale = LWPolygon.Width / LWFeatureImg.Width
	}
	polygonPoint := []types.Coord{
		{extent.MinX, extent.MinY},
		{extent.MinX, extent.MaxY},
		{extent.MaxX, extent.MinY},
		{extent.MaxX, extent.MaxY},
	}
	featureImgCentroid := calculateCentroid(rasterPoints)
	polygonCentroid := calculateCentroid(polygonPoint)

	x = featureImgCentroid[0]
	y = featureImgCentroid[1]

	// Calculate world file parameters

	var p types.WorldFileParameter
	p.A = scale * math.Cos(radian)
	p.D = scale * math.Sin(radian)
	p.B = scale * math.Sin(radian)
	p.E = -scale * math.Cos(radian)
	p.C = polygonCentroid[0] - p.A*(x) - p.B*(y)
	p.F = polygonCentroid[1] - p.D*(x) - p.E*(y)

	// fmt.Println("\nA : ", p.A)
	// fmt.Println("D : ", p.D)
	// fmt.Println("B : ", p.B)
	// fmt.Println("E : ", p.E)
	// fmt.Println("C : ", p.C)
	// fmt.Println("F : ", p.F)
	// fmt.Println("\nAlpha : ", angle)
	// fmt.Println("sin a : ", math.Sin(radian))
	// fmt.Println("cos a : ", math.Cos(radian))
	return &p
}
func GetImageDimensions(file io.Reader) (types.Dimension, error) {
	time.Sleep(2 * time.Second)
	imgConfig, _, err := image.DecodeConfig(file)
	if err != nil {
		fmt.Println(err.Error())
		return types.Dimension{}, fmt.Errorf("Error decoding image config: %s", err)
	}
	return types.Dimension{
		Length: float64(imgConfig.Width),
		Width:  float64(imgConfig.Height),
	}, nil
}
func GetOrientationTag(file io.Reader) (int, error) {
	time.Sleep(2 * time.Second)

	x, err := exif.Decode(file)
	if err != nil {
		return 0, err
	}

	o, err := x.Get(exif.Orientation) // normally, don't ignore errors!
	if err != nil {
		var tagNotPresentError exif.TagNotPresentError = "Orientation"
		if err.Error() == tagNotPresentError.Error() {
			return 0, nil
		}
		return 0, err
	}
	orientation, err := o.Int(0)
	if err != nil {
		return 0, err
	}
	return orientation, nil
}

func GetOrientedImageDimensions(file1 io.Reader, file2 io.Reader) (types.Dimension, error) {
	imgDim, err := GetImageDimensions(file1)
	if err != nil {
		return types.Dimension{}, fmt.Errorf("Error GetImageDimensions : %w", err)
	}
	oValue, err := GetOrientationTag(file2)
	fmt.Println("orientation value : ", oValue)
	fmt.Println("orientation before : ", imgDim)
	if err != nil {
		return types.Dimension{}, fmt.Errorf("Error GetOrientationTag : %w", err)
	}
	if oValue == 8 || oValue == 6 { // Swap width and height if rotation 90 or -90 degree
		imgDim.Length, imgDim.Width = imgDim.Width, imgDim.Length
	}
	fmt.Println("orientation after : ", imgDim)

	//return imgDim, fmt.Errorf("error testing ########################")
	return imgDim, nil
}
func GetOrientedImageTesting(file multipart.File) (types.Dimension, error) {
	// pipeReader, pipeWriter := io.Pipe()
	// bodyReader := io.TeeReader(file, pipeWriter)

	imgDim, err := GetImageDimensions(file)
	fmt.Println("function 1")
	if err != nil {
		return types.Dimension{}, fmt.Errorf("Error decoding image config: %s", err)
	}

	oValue, err := GetOrientationTag(file)
	fmt.Println("function 2")
	if err != nil {
		fmt.Println(oValue)
		return types.Dimension{}, fmt.Errorf("Error Get Orientation Tag: %s", err)
	}
	if oValue == 8 || oValue == 6 { // Swap width and height if rotation 90 or -90 degree
		imgDim.Length, imgDim.Width = imgDim.Width, imgDim.Length
	}
	return imgDim, nil
	//return imgDim, fmt.Errorf("error testing")
}
func WriteWorldFileParametersToFile(filePath string, p types.WorldFileParameter) error {
	content := fmt.Sprintf("%.20f\n%.20f\n%.20f\n%.20f\n%.20f\n%.20f\n", p.A, p.D, p.B, p.E, p.C, p.F)
	err := ioutil.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return err
	}
	return nil
}
func GetRasterFeaturePoints(filePath string) ([]types.Coord, error) {
	cmd := exec.Command("python", "-c", fmt.Sprintf("import pypy; print(pypy.rasterFeaturePoints('%s',True))", strings.Replace(filePath, "\\", "/", 10)))
	fmt.Println(cmd.Args)
	rasterFeaturePoints, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Failed to call python function. error : %s.", err.Error())
	}

	var points types.FeaturePoints
	//fmt.Println(rasterFeaturePoints)
	err = json.Unmarshal(rasterFeaturePoints, &points)
	if err != nil {
		return nil, fmt.Errorf("Failed to Unmarshal rasterFeaturePoints. error : %s.", err.Error())
	}
	return points.Points, nil
}
func RemoveAll(dir string) error {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.RemoveAll(path)
	})
	if err != nil {
		return err
	}
	return nil
}

func SaveFile(filePath string, fileHeader *multipart.FileHeader) error {
	file, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("Failed to open file. error : %s.", err.Error())
	}
	defer file.Close()

	// Create a new file in the server's "uploads" directory

	newFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("Failed to create file. error : %s.", err.Error())
	}
	defer newFile.Close()

	// Copy the uploaded file's contents to the new file
	_, err = io.Copy(newFile, file)
	if err != nil {
		return fmt.Errorf("Failed to copy file contents. error : %s.", err.Error())
	}
	return nil
}
func FileNameWithoutExtension(fileName string) string {
	return fileName[:len(fileName)-len(path.Ext(fileName))]
}
func AllNotNil(data ...any) bool {
	for _, d := range data {
		if d == nil || d == "" {
			return false
		}
	}
	return true
}
func ZipDirectory(source string, zipWriter *zip.Writer, basePath string) error {
	files, err := os.ReadDir(source)
	if err != nil {
		return err
	}

	for _, file := range files {
		filePath := filepath.Join(source, file.Name())
		relPath, _ := filepath.Rel(basePath, filePath)

		if file.IsDir() {
			if err := ZipDirectory(filePath, zipWriter, basePath); err != nil {
				return err
			}
		} else {
			fileWriter, err := zipWriter.Create(relPath)
			if err != nil {
				return err
			}

			fileReader, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer fileReader.Close()

			if _, err := io.Copy(fileWriter, fileReader); err != nil {
				return err
			}
		}
	}
	return nil
}
func CalculateRotationAngle(point1, point2 types.Coord) float64 {
	// Calculate the differences in x and y coordinates
	x1, y1 := point1[0], point1[1]
	x2, y2 := point2[0], point2[1]

	dx := x2 - x1
	dy := y2 - y1

	// Calculate the angle using the arctangent function (Atan2)
	// Note that Atan2 returns the angle in radians, so we convert it to degrees.
	angleRad := math.Atan2(dy, dx)
	angleDeg := angleRad * 180.0 / math.Pi

	// Ensure the angle is between 0 and 360 degrees
	if angleDeg < 0 {
		angleDeg += 360.0
	}

	return angleDeg
}

func FindDiagonalPoints(rectangle []types.Coord) (types.Diagonal, error) {
	if len(rectangle) != 4 {
		return types.Diagonal{}, fmt.Errorf("rectangle must have exactly 4 points")
	}

	// Find the two points with the smallest sum of X and Y coordinates (top-left and bottom-right)
	topLeft, bottomRight := rectangle[0], rectangle[0]
	// Find the two points with the smallest difference of X and Y coordinates (top-right and bottom-left)
	topRight, bottomLeft := rectangle[0], rectangle[0]
	for _, point := range rectangle {
		if point[0]+point[1] < bottomLeft[0]+bottomLeft[1] {
			bottomLeft = point //bottomLeft
		}
		if point[0]+point[1] > topRight[0]+topRight[1] {
			topRight = point //topRight
		}

		if point[0]-point[1] < topLeft[0]-topLeft[1] {
			topLeft = point //topLeft
		}
		if point[0]-point[1] > bottomRight[0]-bottomRight[1] {
			bottomRight = point //bottomRight
		}
	}

	// The diagonal points
	diagonal := types.Diagonal{
		TopLeft:     bottomLeft,
		TopRight:    bottomRight,
		BottomLeft:  topLeft,
		BottomRight: topRight,
	}
	return diagonal, nil
}

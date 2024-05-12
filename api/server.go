package api

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/nahrx/geomatis-api/storage"
	"github.com/nahrx/geomatis-api/types"
	"github.com/nahrx/geomatis-api/util"
)

type Server struct {
	listenAddr string
	store      storage.Storage
}
type ApiError struct {
	Error string `json:"error"`
}
type ApiSuccess struct {
	Message string `json:"message"`
}
type apiFunc func(http.ResponseWriter, *http.Request) error

func makeHttpHandleFunc(f apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			WriteJson(w, http.StatusBadRequest, ApiError{Error: err.Error()})
		}
	}
}
func AddCorsHeader(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}
func WriteJson(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	AddCorsHeader(w)
	w.WriteHeader(status)
	fmt.Println(v)
	return json.NewEncoder(w).Encode(v)
}

func ReqVars(r *http.Request) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.NewDecoder(r.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func NewServer(listenAddr string, store storage.Storage) *Server {
	return &Server{
		listenAddr: listenAddr,
		store:      store,
	}
}

func (s *Server) Start() error {
	r := mux.NewRouter()
	r.HandleFunc("/master-maps", makeHttpHandleFunc(s.handleMasterMaps))
	r.HandleFunc("/master-maps/{name}", makeHttpHandleFunc(s.handleMasterMapsByName))
	r.HandleFunc("/master-maps/{name}/attributes", makeHttpHandleFunc(s.handleMasterMapAttributes))
	r.HandleFunc("/georeference", makeHttpHandleFunc(s.handleGeoreference))
	r.HandleFunc("/repos", makeHttpHandleFunc(s.handleRepos))
	r.HandleFunc("/exports", makeHttpHandleFunc(s.handleExports))

	return http.ListenAndServe(s.listenAddr, r)
}

func (s *Server) handleRepos(w http.ResponseWriter, r *http.Request) error {
	fmt.Println(r.Method)
	switch r.Method {
	case "POST":
		return s.handleGetRepos(w, r)
	case "DELETE":
		return s.handleDeleteRepos(w, r)
	case "PUT":
		return s.handleUpdateRepos(w, r)
	case "OPTIONS":
		return WriteJson(w, http.StatusOK, ApiSuccess{Message: "OPTIONS return successfully"})
	}
	//return WriteJson(w, http.StatusMethodNotAllowed, ApiError{Error: "Method not allowed"})
	return fmt.Errorf("Method not allowed")
}

type Dir struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

func (s *Server) handleGetRepos(w http.ResponseWriter, r *http.Request) error {
	vars, err := ReqVars(r)
	if err != nil {
		return fmt.Errorf("error ReqVars : %w", err)
	}
	vPath, ok := vars["path"]
	if !ok {
		vPath = ""
	}
	path := filepath.Join("uploads", vPath.(string))

	pathInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error os.Stat : %w", err)
	}
	if !pathInfo.IsDir() {
		return fmt.Errorf("%s is not a directory.", path)
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("error os.ReadDir : %w", err)
	}
	var list []Dir
	for _, file := range files {
		info, _ := file.Info()
		file := Dir{
			Name:  file.Name(),
			IsDir: file.IsDir(),
			Size:  info.Size(),
		}
		list = append(list, file)
	}
	return WriteJson(w, http.StatusOK, list)
}
func (s *Server) handleDeleteRepos(w http.ResponseWriter, r *http.Request) error {
	vars, err := ReqVars(r)
	if err != nil {
		return err
	}

	vPath := vars["path"]
	if !util.AllNotNil(vPath) {
		return fmt.Errorf("API parameter is not complete.")
	}

	path := filepath.Join("uploads", vPath.(string))
	fmt.Println(path)
	err = os.RemoveAll(path)
	if err != nil {
		return err
	}
	return WriteJson(w, http.StatusOK, ApiSuccess{Message: "Delete successfully"})
}
func (s *Server) handleUpdateRepos(w http.ResponseWriter, r *http.Request) error {
	//defer r.Body.Close()
	vars, err := ReqVars(r)
	if err != nil {
		return err
	}
	vPath := vars["path"]
	vNewPath := vars["new_path"]
	vMethod := vars["method"]

	if !util.AllNotNil(vPath, vNewPath, vMethod) {
		return fmt.Errorf("API parameter is not complete.")
	}
	path := filepath.Join("uploads/", vPath.(string))
	newPath := filepath.Join("uploads/", vNewPath.(string))
	method := vMethod.(string)

	if method != "rename" {
		return fmt.Errorf("method for updating repository not allowed")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist")
	}
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		return fmt.Errorf("newPath already exist")
	}
	if path == newPath {
		return fmt.Errorf("path is not changed")
	}
	err = os.Rename(path, newPath)
	if err != nil {
		return err
	}
	return WriteJson(w, http.StatusOK, ApiSuccess{Message: "rename successfully"})
}
func (s *Server) handleExports(w http.ResponseWriter, r *http.Request) error {
	fmt.Println(r.Method)
	switch r.Method {
	case "POST":
		return s.handleDownload(w, r)
	}
	//return WriteJson(w, http.StatusMethodNotAllowed, ApiError{Error: "Method not allowed"})
	return fmt.Errorf("Method not allowed")
}
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) error {
	vars, err := ReqVars(r)
	vPath := vars["path"]
	if err != nil {
		return err
	}
	if !util.AllNotNil(vPath) {
		return fmt.Errorf("API parameter is not complete.")
	}
	path := filepath.Join("uploads", vPath.(string))
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("File or directory not found")
	}

	AddCorsHeader(w)
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/octet-stream")

	if fileInfo.IsDir() {
		// Zip the directory and serve as download
		zipFileName := fileInfo.Name() + ".zip"
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFileName))
		zipWriter := zip.NewWriter(w)
		defer zipWriter.Close()

		err = util.ZipDirectory(path, zipWriter, path)
		if err != nil {
			return fmt.Errorf("Error ZipDirectory : %w", err)
		}
	} else {
		// Serve the file directly
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileInfo.Name()))

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("Error os.Open : %w", err)
		}
		defer file.Close()

		_, err = io.Copy(w, file)
		if err != nil {
			return fmt.Errorf("Error os.Copy : %w", err)
		}
	}

	return nil
}
func (s *Server) handleGeoreference(w http.ResponseWriter, r *http.Request) error {
	fmt.Println(r.Method)
	switch r.Method {
	case "POST":
		return s.handleCreateWorldFiles(w, r)
	}
	//return WriteJson(w, http.StatusMethodNotAllowed, ApiError{Error: "Method not allowed"})
	return fmt.Errorf("Method not allowed")
}
func (s *Server) handleCreateWorldFiles(w http.ResponseWriter, r *http.Request) error {
	var maxRequestBodySize int64 = 300 << 20 // 10*2^20 = 10 MB (pembulatan)
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	// _, err := r.MultipartReader()
	// if err != nil {
	// 	return fmt.Errorf("Error MultipartReader : ,%w", err)
	// }
	defer r.Body.Close()

	geoRequest, err := s.NewGeoreferenceRequest(r)
	if err != nil {
		return err
	}
	geoSettings := geoRequest.Settings
	//Create Directory
	uuidPath := uuid.NewString()
	//dirPath := filepath.Join(basePath, uuidPath)
	fmt.Println(uuidPath)
	dirPath := filepath.Join(geoSettings.TargetDir)
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create directory %s. error : %s.", dirPath, err.Error())
	}
	success, fail, err := s.GeoreferenceRasterFiles(geoRequest)

	var errMsg = ""
	if err != nil {
		errMsg = err.Error()
	}
	response := Georeference_response{
		Dir_path: geoSettings.TargetDir,
		Success:  success,
		Fail:     fail,
		Err:      errMsg,
	}
	return WriteJson(w, http.StatusOK, response)
}

type Georeference_response struct {
	Dir_path string `json:"dir_path"`
	Success  int    `json:"success"`
	Fail     int    `json:"fail"`
	Err      string `json:"error"`
}

func GetWorldFileExtlist() map[string]string {
	return map[string]string{
		".jpg":  ".jgw",
		".jpeg": ".jgw",
		".png":  ".pgw",
	}
}
func NewRasterKeySettings(category, prefixNumChar, suffixNumChar, regex string) (*types.RasterKeySettings, error) {
	var err error
	var rasterKey types.RasterKeySettings
	rasterKey.Type = category
	switch category {
	case "all":
		break
	case "prefix":
		rasterKey.NumChar, err = strconv.Atoi(prefixNumChar)
		if err != nil {
			return nil, fmt.Errorf("raster_key_prefix_num_char, Number of prefix raster key characters is not valid. error : %s.", err.Error())
		}
		if rasterKey.NumChar <= 0 {
			return nil, fmt.Errorf("raster_key_prefix_num_char, Number of prefix raster key characters is not valid.")
		}
		break
	case "suffix":
		rasterKey.NumChar, err = strconv.Atoi(suffixNumChar)
		if err != nil {
			return nil, fmt.Errorf("raster_key_suffix_num_char, Number of suffix Raster key characters is not valid. error : %s.", err.Error())
		}
		if rasterKey.NumChar <= 0 {
			return nil, fmt.Errorf("raster_key_prefix_num_char, Number of prefix raster key characters is not valid.")
		}
		break
	case "regex":
		rasterKey.Regex, err = regexp.Compile(regex)
		if err != nil {
			return nil, fmt.Errorf("Raster key regex is not valid. error : %s.", err.Error())
		}
		break
	default:
		return nil, fmt.Errorf("Type of raster key is not valid. Only all, prefix, suffix, or regex allowed.")
	}
	return &rasterKey, nil
}
func GetRasterKey(filename string, rasterKeySettings *types.RasterKeySettings) (string, error) {
	filename = util.FileNameWithoutExtension(filename)
	if len(filename) < rasterKeySettings.NumChar {
		return "", fmt.Errorf("Length of string is not enough (less than rasterKeySettings.NumChar)")
	}
	switch rasterKeySettings.Type {
	case "all":
		return filename, nil
	case "prefix":
		return filename[:rasterKeySettings.NumChar], nil
	case "suffix":
		return filename[len(filename)-rasterKeySettings.NumChar:], nil
	case "regex":
		return rasterKeySettings.Regex.FindString(filename), nil
	}
	return "", fmt.Errorf("Type of raster key is not valid. Only all, prefix, suffix, or regex allowed.")
}
func NewRasterFeatureSettings(xPosition, yPosition, margin string) (*types.RasterFeatureSettings, error) {
	var marginfloat64 float64
	marginfloat64, err := strconv.ParseFloat(margin, 64)
	if err != nil {
		return nil, fmt.Errorf("Feature margin is not valid. error : %s", err)
	}
	return &types.RasterFeatureSettings{
		XPosition: xPosition,
		YPosition: yPosition,
		Margin:    marginfloat64,
	}, nil
}

func (s *Server) NewGeoreferenceRequest(r *http.Request) (*types.GeoreferenceRequest, error) {
	err := r.ParseMultipartForm(300 << 20) // grab the multipart form
	if err != nil {
		return nil, fmt.Errorf("Error ParseMultipartForm : %w", err)
	}

	multipartForm := r.MultipartForm

	masterMap := r.FormValue("master_map")
	attrKey := r.FormValue("attr_key")
	rasterKeyType := r.FormValue("raster_key_type")
	rasterKeyPrefixNumChar := r.FormValue("raster_key_prefix_num_char")
	rasterKeySuffixNumChar := r.FormValue("raster_key_suffix_num_char")
	rasterKeyRegex := r.FormValue("raster_key_regex")
	targetDir := r.FormValue("target_dir")
	separateDir := r.FormValue("separate_dir")
	featureXPosition := r.FormValue("feature_x_position")
	featureYPosition := r.FormValue("feature_y_position")
	featureMargin := r.FormValue("feature_margin")

	fmt.Println(masterMap)
	fmt.Println(attrKey)
	fmt.Println(rasterKeyType)
	fmt.Println(targetDir)
	fmt.Println(separateDir)

	if !util.AllNotNil(multipartForm) {
		return nil, fmt.Errorf("API parameter is not complete.")
	}

	rasters := multipartForm.File["rasters"]

	if len(rasters) == 0 {
		return nil, fmt.Errorf("missing rasters file")
	}
	if masterMap == "" {
		return nil, fmt.Errorf("missing master_map parameter")
	}
	if attrKey == "" {
		return nil, fmt.Errorf("missing attr_key parameter")
	}
	if rasterKeyType == "" {
		return nil, fmt.Errorf("missing raster_key_type parameter")
	}

	masterMapExist, err := s.store.MasterMapExist(masterMap)
	if err != nil {
		return nil, fmt.Errorf("Error when calling MasterMapExist. Error :  %s", err.Error())
	}
	if !masterMapExist {
		return nil, fmt.Errorf("%s is not found in the database. Error :  %s", masterMap, err.Error())
	}

	attrKeyExist, err := s.store.MasterMapAttributeExist(masterMap, attrKey)
	if err != nil {
		return nil, fmt.Errorf("Error when calling MasterMapExist. Error :  %s", err.Error())
	}
	if !attrKeyExist {
		return nil, fmt.Errorf("%s is not found in the database. Error :  %s", masterMap, err.Error())
	}
	rasterKey, err := NewRasterKeySettings(rasterKeyType, rasterKeyPrefixNumChar, rasterKeySuffixNumChar, rasterKeyRegex)
	if err != nil {
		return nil, fmt.Errorf("Error when calling NewRasterKeySettings. Error :  %s", err.Error())
	}

	var separateDirArray []string
	if separateDir == "" {
		separateDir = "[]"
	}

	if err := json.Unmarshal([]byte(separateDir), &separateDirArray); err != nil {
		return nil, fmt.Errorf("Error Unmarshal separateDir Json. Error : %s", err.Error())
	}

	regexValidDir, err := regexp.Compile("([a-zA-Z0-9 \\/])\\w+")
	if err != nil {
		return nil, fmt.Errorf("Error regexp.Compile. Error : %s", err.Error())
	}
	targetDir = filepath.Join("uploads", targetDir)
	if !regexValidDir.MatchString(targetDir) {
		return nil, fmt.Errorf("Target directory name must be valid.")
	}

	rasterFeature, err := NewRasterFeatureSettings(featureXPosition, featureYPosition, featureMargin)
	if err != nil {
		return nil, fmt.Errorf("Error when calling NewRasterFeatureSettings. Error : %s", err.Error())
	}
	return &types.GeoreferenceRequest{
		Raster: rasters,
		Settings: &types.GeoreferenceSettings{
			MasterMap:             masterMap,
			AttrKey:               attrKey,
			RasterKeySettings:     rasterKey,
			TargetDir:             targetDir,
			SeparateDirAttrs:      separateDirArray,
			RasterFeatureSettings: rasterFeature,
		},
	}, nil
}

func (s *Server) GeoreferenceRasterFiles_backup(g *types.GeoreferenceRequest) error {
	var e error = nil
	gSettings := g.Settings
	for _, fileHeader := range g.Raster {
		//Get image dimension
		file1, err := fileHeader.Open()
		if err != nil {
			err = fmt.Errorf("Error while converting multipart FileHeader to File. error : %s.", err.Error())
			e = fmt.Errorf("%w\n %w", e, err)
			break
		}
		file2, err := fileHeader.Open()
		if err != nil {
			err = fmt.Errorf("Error while converting multipart FileHeader to File. error : %s.", err.Error())
			e = fmt.Errorf("%w\n %w", e, err)
			break
		}

		// img, err := util.GetOrientedImageDimensions(file)
		// if err != nil {
		// 	err = fmt.Errorf("Error GetImageDimensions : %s.", err.Error())
		// 	e = fmt.Errorf("%w\n %w", e, err)
		// 	break
		// }
		imgDim, err := util.GetOrientedImageDimensions(file1, file2)

		if err != nil {
			err = fmt.Errorf("Error GetOrientationTag : %w", err)
			e = fmt.Errorf("%w\n %w", e, err)
			break
		}
		//Get raster key
		rasterKey, err := GetRasterKey(fileHeader.Filename, gSettings.RasterKeySettings)
		if err != nil {
			err = fmt.Errorf("Error GetRasterKey: %s.", err.Error())
			e = fmt.Errorf("%w\n %w", e, err)
			break
		}

		//Get separateDir attributes and save file
		separateDirName, err := s.store.GetAttributesValue(gSettings.MasterMap, gSettings.AttrKey, rasterKey, gSettings.SeparateDirAttrs)
		if err != nil {
			err = fmt.Errorf("Error GetAttributesValue : %s.", err.Error())
			e = fmt.Errorf("%w\n %w", e, err)
			break
		}

		dir := strings.Join(separateDirName, "/")
		targetDir := filepath.Join(gSettings.TargetDir, dir)
		filePath := filepath.Join(targetDir, fileHeader.Filename)

		if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
			err = fmt.Errorf("Failed to create directory %s. error : %s.", targetDir, err.Error())
			e = fmt.Errorf("%w\n %w", e, err)
			break
		}

		err = util.SaveFile(filePath, fileHeader)
		if err != nil {
			err = fmt.Errorf("Failed to save file. error : %s.", err.Error())
			e = fmt.Errorf("%w\n %w", e, err)
			break
		}
		//Get polygon extent, raster feature point from image

		polygonExtent, err := s.store.GetExtent(gSettings.MasterMap, rasterKey)
		if err != nil {
			err = fmt.Errorf("Error GetExtent : %s.", err.Error())
			e = fmt.Errorf("%w\n %w", e, err)
			break
		}

		featurePoints, err := util.GetRasterFeaturePoints(filePath)
		if err != nil {
			err = fmt.Errorf("Error GetRasterFeaturePoints : %s.", err.Error())
			e = fmt.Errorf("%w\n %w", e, err)
			break
		}

		//Calculate Georeference Parameter and save world file
		parameter := util.CalculateGeoreferenceParameters(imgDim, featurePoints, *polygonExtent, gSettings.RasterFeatureSettings.Margin)
		worldFileExt := GetWorldFileExtlist()[strings.ToLower(path.Ext(fileHeader.Filename))]
		fmt.Println("worldFileExt : ", worldFileExt)
		worldFileName := fmt.Sprintf("%s%s", util.FileNameWithoutExtension(filePath), worldFileExt)
		err = util.WriteWorldFileParametersToFile(worldFileName, *parameter)
		if err != nil {
			err = fmt.Errorf("Error while creating worldfile. error : %s.", err.Error())
			e = fmt.Errorf("%w\n %w", e, err)
			break
		}
	}
	return e
}
func (s *Server) worker(id int, gRaster <-chan *multipart.FileHeader, results chan<- types.Result, g *types.GeoreferenceSettings) {
	fmt.Println("worker : ", id)
	for fileHeader := range gRaster {
		result := types.Result{
			Id:    fileHeader.Filename,
			Error: nil,
		}
		//Get image dimension
		file1, err := fileHeader.Open()
		if err != nil {
			result.Error = fmt.Errorf("Error while converting multipart FileHeader to File. error : %s.", err.Error())
			results <- result
			continue
		}
		file2, err := fileHeader.Open()
		if err != nil {
			result.Error = fmt.Errorf("Error while converting multipart FileHeader to File. error : %s.", err.Error())
			results <- result
			continue
		}

		// img, err := util.GetOrientedImageDimensions(file)
		// if err != nil {
		// 	err = fmt.Errorf("Error GetImageDimensions : %s.", err.Error())
		// 	e = fmt.Errorf("%w\n %w", e, err)
		// 	break
		// }
		imgDim, err := util.GetOrientedImageDimensions(file1, file2)

		if err != nil {
			result.Error = fmt.Errorf("Error GetOrientationTag : %w", err)
			results <- result
			continue
		}
		//Get raster key
		rasterKey, err := GetRasterKey(fileHeader.Filename, g.RasterKeySettings)
		if err != nil {
			result.Error = fmt.Errorf("Error GetRasterKey: %s.", err.Error())
			results <- result
			continue
		}

		//Get separateDir attributes and save file
		separateDirName, err := s.store.GetAttributesValue(g.MasterMap, g.AttrKey, rasterKey, g.SeparateDirAttrs)
		if err != nil {
			result.Error = fmt.Errorf("Error GetAttributesValue : %s.", err.Error())
			results <- result
			continue
		}

		dir := strings.Join(separateDirName, "/")
		targetDir := filepath.Join(g.TargetDir, dir)
		filePath := filepath.Join(targetDir, fileHeader.Filename)

		if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
			result.Error = fmt.Errorf("Failed to create directory %s. error : %s.", targetDir, err.Error())
			results <- result
			continue
		}

		err = util.SaveFile(filePath, fileHeader)
		if err != nil {
			result.Error = fmt.Errorf("Failed to save file. error : %s.", err.Error())
			results <- result
			continue
		}
		//Get polygon extent, raster feature point from image

		polygonExtent, err := s.store.GetExtent(g.MasterMap, rasterKey)
		if err != nil {
			result.Error = fmt.Errorf("Error GetExtent : %s.", err.Error())
			results <- result
			continue
		}

		featurePoints, err := util.GetRasterFeaturePoints(filePath)
		if err != nil {
			result.Error = fmt.Errorf("Error GetRasterFeaturePoints : %s.", err.Error())
			results <- result
			continue
		}

		//Calculate Georeference Parameter and save world file
		parameter := util.CalculateGeoreferenceParameters(imgDim, featurePoints, *polygonExtent, g.RasterFeatureSettings.Margin)
		worldFileExt := GetWorldFileExtlist()[strings.ToLower(path.Ext(fileHeader.Filename))]
		fmt.Println("worldFileExt : ", worldFileExt)
		worldFileName := fmt.Sprintf("%s%s", util.FileNameWithoutExtension(filePath), worldFileExt)
		err = util.WriteWorldFileParametersToFile(worldFileName, *parameter)
		if err != nil {
			result.Error = fmt.Errorf("Error while creating worldfile. error : %s.", err.Error())
			results <- result
			continue
		}
		results <- result
	}
}
func (s *Server) GeoreferenceRasterFiles(g *types.GeoreferenceRequest) (int, int, error) {
	numJobs := len(g.Raster)
	numWorkers := 20
	if numJobs < numWorkers {
		numWorkers = numJobs
	}
	fileHeader := make(chan *multipart.FileHeader, numJobs)
	results := make(chan types.Result, numJobs)
	for w := 0; w < numWorkers; w++ {
		go s.worker(w, fileHeader, results, g.Settings)
	}

	for j := 0; j < numJobs; j++ {
		fileHeader <- g.Raster[j]
	}
	close(fileHeader)

	var e error = nil
	success, fail := 0, 0
	for a := 0; a < numJobs; a++ {
		r := <-results
		if r.Error != nil {
			fail++
			errMsg := fmt.Sprintf("error file %s : %s.", r.Id, r.Error.Error())
			if e == nil {
				e = fmt.Errorf(errMsg)
				continue
			}
			e = fmt.Errorf("%s\n %s", e.Error(), errMsg)
		}
	}
	success = numJobs - fail
	return success, fail, e
}
func (s *Server) handleMasterMaps(w http.ResponseWriter, r *http.Request) error {
	fmt.Println(r.Method)
	switch r.Method {
	case "GET":
		return s.handleGetMasterMaps(w, r)
	case "POST":
		return s.handleCreateMasterMaps(w, r)
	case "OPTIONS":
		return WriteJson(w, http.StatusOK, ApiSuccess{Message: "OPTIONS return successfully"})
		//default:
		//http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	//return WriteJson(w, http.StatusMethodNotAllowed, ApiError{Error: "Method not allowed"})
	return fmt.Errorf("Method not allowed")
}
func (s *Server) handleGetMasterMaps(w http.ResponseWriter, r *http.Request) error {
	data, err := s.store.GetMasterMaps()
	if err != nil {
		return err
	}
	return WriteJson(w, http.StatusOK, data)
}
func (s *Server) handleCreateMasterMaps(w http.ResponseWriter, r *http.Request) error {
	name := r.FormValue("name")
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		return fmt.Errorf("error retrieving formfile. error %s ", err.Error())
	}
	defer file.Close()
	// file validation
	fileName := fileHeader.Filename
	allowedExt := ".geojson"
	if path.Ext(fileName) != allowedExt {
		return fmt.Errorf("The uploaded file must have the following extensions : %s ", allowedExt)
	}
	var maxFileSize int64 = 10_000_000
	if fileHeader.Size > maxFileSize {
		return fmt.Errorf("The uploaded file cannot be larger than %v", maxFileSize)
	}
	// file content processing
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error file content processing. error : %s", err.Error())

	}

	if name == "" {
		name = util.FileNameWithoutExtension(fileName)
	}

	err = s.store.CreateMasterMaps(name, &fileBytes)
	if err != nil {
		return fmt.Errorf("error when storing master maps. error : %s", err.Error())

	}
	return WriteJson(w, http.StatusOK, ApiSuccess{Message: fmt.Sprintf("File %s uploaded and processed successfully", fileName)})
}

func (s *Server) handleMasterMapsByName(w http.ResponseWriter, r *http.Request) error {
	fmt.Println(r.Method)

	switch r.Method {
	case "GET":
		return s.handleGetMasterMapsByName(w, r)
	case "DELETE":
		return s.handleDeleteMasterMap(w, r)
	case "OPTIONS":
		return WriteJson(w, http.StatusOK, ApiSuccess{Message: "OPTIONS return successfully"})
		//default:
		//http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	//return WriteJson(w, http.StatusMethodNotAllowed, ApiError{Error: "Method not allowed"})

	return fmt.Errorf("Method not allowed")
}
func (s *Server) handleGetMasterMapsByName(w http.ResponseWriter, r *http.Request) error {
	masterMap := mux.Vars(r)["name"]
	if !util.AllNotNil(masterMap) {
		return fmt.Errorf("API parameter is not complete.")
	}
	data, err := s.store.GetMasterMapByName(masterMap)
	if err != nil {
		return err
	}
	return WriteJson(w, http.StatusOK, data)
}
func (s *Server) handleDeleteMasterMap(w http.ResponseWriter, r *http.Request) error {
	masterMap := mux.Vars(r)["name"]
	if !util.AllNotNil(masterMap) {
		return fmt.Errorf("request parameter not found. master_maps needed in the request.")
	}
	err := s.store.DeleteMasterMap(masterMap)
	if err != nil {
		return err
	}
	return WriteJson(w, http.StatusOK, ApiSuccess{Message: "Delete successfully"})
}

func (s *Server) handleMasterMapAttributes(w http.ResponseWriter, r *http.Request) error {
	fmt.Println(r.Method)
	switch r.Method {
	case "GET":
		return s.handleGetMasterMapAttributes(w, r)
		//return WriteJson(w, http.StatusMethodNotAllowed, ApiError{Error: "Method not allowed"})

	}
	return fmt.Errorf("Method not allowed")
}
func (s *Server) handleGetMasterMapAttributes(w http.ResponseWriter, r *http.Request) error {
	masterMap := mux.Vars(r)["name"]
	if !util.AllNotNil(masterMap) {
		return fmt.Errorf("API parameter is not complete.")
	}
	data, err := s.store.GetMasterMapAttributes(masterMap)
	if err != nil {
		return err
	}
	return WriteJson(w, http.StatusOK, data)
}

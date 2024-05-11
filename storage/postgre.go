package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"github.com/nahargo/geomatis-api/types"

	"github.com/twpayne/go-geom/encoding/geojson"
)

type PostgreStorage struct {
	Db *sql.DB
}

func makeSqlScanFunc[T comparable](columns []T) []interface{} {
	columnPointers := make([]interface{}, cap(columns))
	for i, _ := range columns {
		columnPointers[i] = &columns[i]
	}
	return columnPointers
}

func NewPostgreStorage() (*PostgreStorage, error) {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbDatabase := os.Getenv("DB_DATABASE")
	dbUsername := os.Getenv("DB_USERNAME")
	dbPassword := os.Getenv("DB_PASSWORD")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s database=%s sslmode=disable", dbHost, dbPort, dbUsername, dbPassword, dbDatabase)
	//connStr := "postgres://postgres:password@localhost/geomatis?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &PostgreStorage{
		Db: db,
	}, nil
}
func (s *PostgreStorage) TableExist(tableName string) (bool, error) {
	// Retrieve table names from the database
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			AND table_name = $1
		)
	`
	var exists bool
	err := s.Db.QueryRow(query, tableName).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
func (s *PostgreStorage) MasterMapExist(masterMap string) (bool, error) {
	// Retrieve table names from the database
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM geometry_columns
			WHERE f_table_schema = 'public'
			AND f_table_name = $1
		)
	`
	var exists bool
	err := s.Db.QueryRow(query, masterMap).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
func (s *PostgreStorage) MasterMapAttributeExist(masterMap, attr string) (bool, error) {
	query := `
	SELECT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_name = $1 AND column_name = $2
	)`
	var exists bool
	err := s.Db.QueryRow(query, masterMap, attr).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (s *PostgreStorage) GetMasterMaps() ([]types.MasterMap, error) {
	// Retrieve table names from the database
	query, err := s.Db.Query(`
		SELECT f_table_name AS name, coord_dimension AS dimension, srid, type 
		FROM geometry_columns 
		WHERE f_table_schema='public'
	`)
	if err != nil {
		return nil, err
	}
	defer query.Close()
	var values []types.MasterMap
	for query.Next() {
		var v types.MasterMap
		err := query.Scan(&v.Name, &v.Dimension, &v.Srid, &v.Category)
		if err != nil {
			return nil, err
		}

		values = append(values, v)
	}
	if err := query.Err(); err != nil {
		return nil, err
	}

	// Create the response object
	return values, nil
}
func (s *PostgreStorage) GetMasterMapByName(masterMap string) (types.MasterMap, error) {
	// Retrieve table names from the database
	query := `
			SELECT f_table_name, coord_dimension, srid, type
			FROM geometry_columns
			WHERE f_table_schema = 'public'
			AND f_table_name = $1
	`
	//var v map[string]string
	//err := s.Db.QueryRow(query, masterMap).Scan(&v)
	var v types.MasterMap
	err := s.Db.QueryRow(query, masterMap).Scan(&v.Name, &v.Dimension, &v.Srid, &v.Category)
	if err != nil {
		return types.MasterMap{}, err
	}

	return v, nil
}
func (s *PostgreStorage) GetMasterMapAttributes(masterMap string) ([]types.MasterMapAttr, error) {
	query, err := s.Db.Query(`
			SELECT column_name,udt_name
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE table_schema = 'public' and TABLE_NAME=$1 and udt_name != 'geometry'
			ORDER BY column_name ASC
			`, masterMap)

	if err != nil {
		return nil, err
	}
	defer query.Close()
	var values []types.MasterMapAttr
	for query.Next() {
		var v types.MasterMapAttr
		err := query.Scan(&v.Name, &v.Category)
		if err != nil {
			return nil, err
		}

		values = append(values, v)
	}
	if err := query.Err(); err != nil {
		return nil, err
	}

	// Create the response object
	return values, nil
}

func (s *PostgreStorage) GetExtent(tableName, idSls string) (*types.Extent, error) {

	// Query to get the bounding box coordinates
	query := fmt.Sprintf("SELECT ST_XMin(ST_Extent(geom)), ST_YMin(ST_Extent(geom)), ST_XMax(ST_Extent(geom)), ST_YMax(ST_Extent(geom)) FROM %s WHERE idsls = '%s'", tableName, idSls)

	var minX, minY, maxX, maxY float64
	err := s.Db.QueryRow(query).Scan(&minX, &minY, &maxX, &maxY)
	if err != nil {
		//return nil, fmt.Errorf("error. Error :%s", err.Error())
		return nil, fmt.Errorf("Failed to fetch bounding box from database. Error :%s", err.Error())
	}

	// Create a BoundingBox object with the coordinates
	extent := types.Extent{
		MinX: minX,
		MinY: minY,
		MaxX: maxX,
		MaxY: maxY,
	}

	return &extent, nil
}

func (s *PostgreStorage) GetAttributesValue(table string, attrKey string, key string, attributes []string) ([]string, error) {
	selectQuery := strings.Join(attributes, ",")
	query := fmt.Sprintf(`
	SELECT %s
		FROM %s
		WHERE %s = '%s'
	`, selectQuery, table, attrKey, key)

	// columns := make([]string, len(attributes))
	// columnPointers := make([]interface{}, len(attributes))
	// for i, _ := range columns {
	// 	columnPointers[i] = &columns[i]
	// }

	// err := s.Db.QueryRow(query).Scan(columnPointers...)
	columns := make([]string, len(attributes))
	err := s.Db.QueryRow(query).Scan(makeSqlScanFunc(columns)...)
	if err != nil {
		return nil, err
	}

	return columns, nil

}
func (s *PostgreStorage) CreateMasterMaps(tableName string, fileData *[]byte) error {
	tableExist, err := s.TableExist(tableName)
	if err != nil {
		return fmt.Errorf("Error when checking the table existence (%s) in database. %s", tableName, err.Error())
	}
	if tableExist {
		return fmt.Errorf("Layer name or table (%s) already exists in the database. ", tableName)
	}

	var data geojson.FeatureCollection
	if err := json.Unmarshal(*fileData, &data); err != nil {
		return err
	}
	i := 0
	for _, feature := range data.Features {
		// Convert the GeoJSON geometry to WKT
		//prop, err := json.Marshal(feature.Properties)

		if i == 0 {
			geometry, err := geojson.Encode(feature.Geometry)
			if err != nil {
				return err
			}

			propTypes := constructDataTypes(feature.Properties)
			propTypes["geom"] = fmt.Sprintf("geometry(%v, %v)", geometry.Type, 4326)

			_, err = s.createTable(tableName, propTypes)
			if err != nil {
				return err
			}

			i++
			//return []byte(createQuery), nil
		}

		geom, err := geojson.Marshal(feature.Geometry)
		if err != nil {
			return err
		}

		// Build the INSERT statement
		var data map[string]interface{}
		data = feature.Properties
		data["geom"] = []byte(fmt.Sprintf("ST_SetSRID(ST_GeomFromGeoJSON('%s'), 4326)", geom))
		_, err = s.insertData(tableName, data)
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *PostgreStorage) DeleteMasterMap(masterMap string) error {
	exist, err := s.MasterMapExist(masterMap)
	if err != nil {
		return err
	}
	if !exist {
		return fmt.Errorf("Master maps doesnt exist")
	}

	query := fmt.Sprintf("DROP TABLE IF EXISTS %v", masterMap)

	_, err = s.Db.Exec(query)
	if err != nil {
		return err
	}
	return nil
}
func constructDataTypes(prop map[string]interface{}) map[string]string {
	propTypes := map[string]string{}
	for key, val := range prop {
		if val == nil {
			propTypes[key] = fmt.Sprintf("%T", "")
			continue
		}
		if key == "geom" {
			key = "__geom"
		}
		propTypes[key] = fmt.Sprintf("%T", val)

		// UNTUK MEMBEDAKAN FLOAT64 DAN INT64

		// rawType := fmt.Sprintf("%T", val)
		// if rawType != "float64" {
		// 	propTypes[key] = fmt.Sprintf("%T", val)
		// 	continue
		// }

		// s := fmt.Sprintf("%v", val)
		// i, err := strconv.ParseInt(s, 10, 64)
		// if err == nil {
		// 	propTypes[key] = fmt.Sprintf("%T", i)
		// 	continue
		// }
		// f, err := strconv.ParseFloat(s, 64)
		// if err == nil {
		// 	propTypes[key] = fmt.Sprintf("%T", f)
		// 	continue
		// }

	}
	return propTypes
}
func (s *PostgreStorage) createTable(name string, attr map[string]string) (string, error) {
	// Construct Query statement
	query := `CREATE TABLE ` + name + ` (
		gid serial primary key,`
	i := 0
	for key, val := range attr {
		var dType string
		switch val {
		case "float64":
			dType = "numeric"
		case "int64":
			dType = "integer"
		case "string":
			dType = "varchar(254)"
		default:
			dType = val
		}
		if key == "gid" {
			key = "__gid"
		}
		query = fmt.Sprintf(query+"%v %v", key, dType)
		if i < len(attr)-1 {
			query = query + ","
		}

		i++
	}
	query = query + ");"

	// Execute the CREATE table
	_, err := s.Db.Exec(query)
	if err != nil {
		return "", err
	}
	return query, nil
}
func (s *PostgreStorage) insertData(tableName string, data map[string]interface{}) (string, error) {
	query := `INSERT INTO ` + tableName + ` (%v) VALUES (%v)`
	i := 0
	insertInto := ""
	values := ""
	for key, val := range data {
		if key == "gid" {
			key = "__gid"
		}

		var dType string
		dType = fmt.Sprintf("%T", val)

		switch dType {
		case "string":
			val = fmt.Sprintf("'%v'", val)
		case "int":
		case "int32":
		case "int64":
		case "float64":
			val = fmt.Sprintf("%v", val)
		}

		if val == nil {
			val = "null"
		}
		insertInto = insertInto + fmt.Sprintf("%v", key)
		values = values + fmt.Sprintf("%s", val)
		if i < len(data)-1 {
			insertInto = insertInto + ","
			values = values + ","
		}
		i++
	}
	query = fmt.Sprintf(query, insertInto, values)
	//insert := fmt.Sprintf("INSERT INTO your_table_name (geometry, name) VALUES (ST_SetSRID(ST_GeomFromGeoJSON('%s'), 4326), $1)", geom)

	// Execute the INSERT statement
	_, err := s.Db.Exec(query)
	if err != nil {
		return query, err
	}
	return query, nil
}

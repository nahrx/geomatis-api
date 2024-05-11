package storage

import "github.com/nahargo/geomatis-api/types"

type Storage interface {
	TableExist(string) (bool, error)
	MasterMapExist(string) (bool, error)
	MasterMapAttributeExist(string, string) (bool, error)
	GetMasterMaps() ([]types.MasterMap, error)
	GetMasterMapByName(string) (types.MasterMap, error)
	GetMasterMapAttributes(string) ([]types.MasterMapAttr, error)
	GetExtent(string, string) (*types.Extent, error)
	GetAttributesValue(string, string, string, []string) ([]string, error)
	CreateMasterMaps(string, *[]byte) error
	DeleteMasterMap(string) error
}

package Nominatim

import (
	"database/sql"
	//"fmt"
	_ "github.com/lib/pq"
	//"log"
	//"strconv"
)

type PlaceLookup struct {
	iMaxRank int
	//langPreOrder
	addressDetails bool
	placeID        int
	langPreOrder   []int
	db             *sql.DB
}

func NewPlaceLookup(db sql.DB) PlaceLookup {
	p := PlaceLookup{db: &db}
	p.addressDetails = false
	return p
}

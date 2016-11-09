package Nominatim

import (
	"database/sql"

	_ "github.com/lib/pq"
	"github.com/ventu-io/slf"

	"errors"
)

type reverseGeocodeImpl struct {
	db *sql.DB
}

/*type DataWithoutDetails struct {
	PlaceID     string
	OsmType     string
	OsmID       string
	Lat         string
	Lon         string
	Langaddress string
	ID          interface{}
	//dataProc    string
	MachineID string
	TimeReq   string
	FullReq   interface{}
}

func (d DataWithoutDetails) String() string {
	var str = "\nplace_id: " + d.PlaceID +
		"\nosm_id: " + d.OsmID +
		"\nosm_type: " + d.OsmType +
		"\nlat: " + d.Lat +
		"\nlon: " + d.Lon +
		"\nlangaddress: " + d.Langaddress +
		"\nmachineID: " + d.MachineID +
		//	"\nfullReq: " + d.FullReq.(string) +
		"\ntimeReq: " + d.TimeReq + "\n"
	return str
}*/

//NewReverseGeocode - create working ReverseGeocode implemenation, using nominatim db
func NewReverseGeocode(sqlOpenStr string) (ReverseGeocode, error) {

	log.Debugf("NewReverseGeocode %s", sqlOpenStr)

	db, err := sql.Open("postgres", sqlOpenStr)
	if err != nil {
		log.Errorf("sql.Open error %s %v", sqlOpenStr, err)
		return nil, err
	}

	var tmp int
	err = db.QueryRow("SELECT 1").Scan(&tmp)
	if err != nil {
		log.Errorf("db.QueryRow %v", err)
		return nil, err
	}

	r := reverseGeocodeImpl{db: db}
	return &r, nil
}

func (r *reverseGeocodeImpl) Close() error {
	err := r.db.Close()
	if err != nil {
		log.Errorf("Close: error %s", err.Error())
		return err
	}
	return nil
}

func zoomToRank(iZoom int) int {
	aZoomRank := map[int]int{
		0:  2,
		1:  2,
		2:  2,
		3:  4,
		4:  4,
		5:  8,
		6:  10,
		7:  10,
		8:  12,
		9:  12,
		10: 17,
		11: 17,
		12: 18,
		13: 18,
		14: 22,
		15: 22,
		16: 26,
		17: 26,
		18: 30,
		19: 30,
	}

	if value, ok := aZoomRank[iZoom]; ok {
		return value
	}
	return 28

	/*if r.iMaxRank = 28; aZoomRank[iZoom] {
		r.iMaxRank = aZoomRank[iZoom]
	}*/
}

func parsePlaceDataMap(m map[string]string) *ReverseGeocodeResponse {
	//TODO: fix types - use float/int64/etc instead of string
	return &ReverseGeocodeResponse{
		PlaceID:     m["place_id"],
		OsmID:       m["osm_id"],
		OsmType:     m["osm_type"],
		Lat:         m["lat"],
		Lon:         m["lon"],
		Langaddress: m["langaddress"],
	}
}

func (r *reverseGeocodeImpl) Lookup(request *ReverseGeocodeRequest) (*ReverseGeocodeResponse, error) {

	//sLon := strconv.FormatFloat(r.fLon, 'f', 6, 64)
	//sLat := strconv.FormatFloat(r.fLat, 'f', 6, 64)

	//log.Println(sPointSQL)

	//var sPointSQL string = "ok"

	var (
		iMaxRank         = zoomToRank(request.Zoom)
		fSearchDiam      = 0.0004
		fMaxAreaDistance = 1.0
	)

	var sSQL string

	var (
		iPlaceID     sql.NullInt64
		iParentPlace sql.NullInt64
		iRank        int
	)

	//log.Printf("Lookup %v\n", r)
	//var hasParentPlace bool = false

	for fSearchDiam < fMaxAreaDistance && !iPlaceID.Valid {

		fSearchDiam = fSearchDiam * 2
		if fSearchDiam > 2 && iMaxRank > 4 {
			iMaxRank = 4
		}
		if fSearchDiam > 1 && iMaxRank > 9 {
			iMaxRank = 8
		}
		if fSearchDiam > 0.8 && iMaxRank > 10 {
			iMaxRank = 10
		}
		if fSearchDiam > 0.6 && iMaxRank > 12 {
			iMaxRank = 12
		}
		if fSearchDiam > 0.2 && iMaxRank > 17 {
			iMaxRank = 17
		}
		if fSearchDiam > 0.1 && iMaxRank > 18 {
			iMaxRank = 18
		}
		if fSearchDiam > 0.008 && iMaxRank > 22 {
			iMaxRank = 22
		}
		if fSearchDiam > 0.001 && iMaxRank > 26 {
			iMaxRank = 26
		}

		sSQL = `select place_id,parent_place_id,rank_search
		            from placex
			WHERE ST_DWithin(ST_SetSRID(ST_Point($1,$2),4326), geometry, $3) and
			      rank_search != 28 and
			      rank_search >= $4 and
			      (name is not null or housenumber is not null) and
			      class not in ('waterway','railway','tunnel','bridge') and
			      indexed_status = 0 and
			      (ST_GeometryType(geometry) not in ('ST_Polygon','ST_MultiPolygon') OR
			       ST_DWithin(ST_SetSRID(ST_Point($1,$2),4326), centroid, $3))
			ORDER BY ST_distance(ST_SetSRID(ST_Point($1,$2),4326), geometry)
			ASC limit 1
			`

		//log.Printf("%f %f %f %d", r.fLon, r.fLat, fSearchDiam, iMaxRank)
		err := r.db.QueryRow(sSQL, request.Lon, request.Lat, fSearchDiam, iMaxRank).Scan(&iPlaceID, &iParentPlace, &iRank)
		switch {
		case err == sql.ErrNoRows:
			//log.Printf("Not found.")
			//return nil, err
			continue
		case err != nil:
			log.WithCaller(slf.CallerShort).Errorf("QueryRow %s", err.Error())
			return nil, err
		default:
			//log.Println("QueryRow result:", iPlaceID, iParentPlace, iRank)
		}
	}

	//log.Println("original place_id:", iPlaceID)
	var iNewPlaceID sql.NullInt64

	if iPlaceID.Valid && iMaxRank < 28 {
		if iRank > 28 && iParentPlace.Valid {
			iPlaceID = iParentPlace
			//log.Debugf("use parent place: %v", iParentPlace)
		}

		sSQL = `select address_place_id
				from place_addressline where place_id = $1
				order by abs(cached_rank_address - $2)
				asc,cached_rank_address desc,isaddress desc,distance desc limit 1
			`
		err := r.db.QueryRow(sSQL, iPlaceID, iMaxRank).Scan(&iNewPlaceID)
		switch {
		case err == sql.ErrNoRows:
			break
		case err != nil:
			log.WithCaller(slf.CallerShort).Errorf("QueryRow: %s", err.Error())
			return nil, err
		default:
			//log.Println("address_place_id:", iNewPlaceID)
			if iNewPlaceID.Valid {
				iPlaceID = iNewPlaceID
			}
		}
	}

	if iPlaceID.Valid {
		placeLookup := NewPlaceLookup(r.db)
		//placeLookup.SetLanguagePreference()
		placeLookup.SetIncludeAddressDetails(request.IncludeDetails)
		placeLookup.SetPlaceID(iPlaceID.Int64)
		dataMap, err := placeLookup.Lookup()
		if err != nil {
			return nil, err
		}
		return parsePlaceDataMap(dataMap), nil
		//return dataMapToStruct(dataMap, r.machineID, resentFullReq, r.fullReq), nil
	}

	return nil, errors.New("place not found")
}

package Nominatim

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/ventu-io/slf"
)

var pwdCurr string = "Nominatim/lib/Nominatim"
var log = slf.WithContext(pwdCurr)

type PlaceLookup struct {
	maxRank int
	//langPreOrder   string
	addressDetails bool
	placeID        int64
	db             *sql.DB
}

func NewPlaceLookup(db *sql.DB) PlaceLookup {
	p := PlaceLookup{db: db}
	p.addressDetails = false
	return p
}

/*func (p *PlaceLookup) SetLanguagePreference() {
	p.langPreOrder = `ARRAY['ru', 'en']`
}*/

func (p *PlaceLookup) SetRank(rank int) {
	p.maxRank = rank
}

func (p *PlaceLookup) SetIncludeAddressDetails(addressDetails bool) {
	p.addressDetails = addressDetails
}

func (p *PlaceLookup) SetPlaceID(placeID int64) {
	p.placeID = placeID
}

func (p *PlaceLookup) setOSMID(req_type string, id int) {
	/*SQL := `select place_id from placex where osm_type = '".pg_escape_string($sType)."' and osm_id = ".(int)$iID." order by type = 'postcode' asc";
	$this->iPlaceID = $this->oDB->getOne($sSQL)`*/
}

/*func (p *PlaceLookup) getAddressNames() {
	var (
		address   []string
		fallback  []string
		classType []string = getClassTypes()
	)

	for line := range address {
		fallback = false
		typeLabel := false
		if data, ok := classType[(line["class"] + ":" + line["type"] + ":" + line["adminLevel"])]; ok {
			break
		} else if data, ok := classType[(line["class"] + ":" + line["type"])]; ok {
			break
		} else {
			rankAddress := strconv.FormatInt(line["rank_address"], 10) / 2
			if data, ok := classType["boundary:administrative:" + strconv.Itoa(rankAddress)]; ok {
				typeLabel = data
				fallback = true
			} else {
				typeLabel["simplelabel"] = "address" + line["rank_adress"]
				fallback = true
			}
		}
	}

}*/

/*type sqlReply struct {
	placeID    []string `place_id`
	address    []string `langaddress`
	class      []string `class `
	mainType   []string `type`
	adminLevel []string `admin_level`
}*/

var addressType string

func (p *PlaceLookup) Lookup() (placeData map[string]string) {

	//var (
	//	place_id string
	//	address     []string
	//	class       []string
	//	mainType    []string
	//	adminLevel  []string
	//	addressType []string
	//)

	//languagePrefArraySQL := p.langPreOrder
	//log.Println(p.langPreOrder, p.pl)

	sqlReq := `select placex.place_id, partition, osm_type, osm_id, class, type, admin_level, housenumber, street, isin, postcode, country_code, extratags,
		parent_place_id, linked_place_id, rank_address, rank_search,
		coalesce(importance,0.75-(rank_search::float/40))
	as importance, indexed_status, indexed_date, wikipedia, calculated_country_code,
		get_address_by_language(place_id, ARRAY['ru','en']) as langaddress,
		get_name_by_language(name, ARRAY['ru','en']) as placename,
		get_name_by_language(name, ARRAY['ref']) as ref,
		st_y(centroid) as lat, st_x(centroid) as lon
	from placex where place_id = $1
	`
	rows, err := p.db.Query(sqlReq, p.placeID)
	if err != nil {
		log.Errorf("Error: %v", err.Error())
		return
	}
	defer rows.Close()

	placeData = map[string]string{}
	//return placeData

	columns, _ := rows.Columns()
	count := len(columns)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)

	//final_result := map[int]map[string]string{} //then more than 1 row
	//result_id := 0

	rows.Next()

	for colNum, _ := range columns {
		valuePtrs[colNum] = &values[colNum]
	}

	rows.Scan(valuePtrs...)

	for colNum, col := range columns {
		var v interface{}
		val := values[colNum]
		b, ok := val.([]byte)
		if ok {
			v = string(b)
		} else {
			v = val
		}
		placeData[col] = fmt.Sprintf("%v", v)
	}

	/*for keyIn, wordIn := range placeData {
		if wordIn != "<nil>" {
			fmt.Print(keyIn, ": ", wordIn, "\n")
		}
	}*/

	/*if p.addressDetails == true {
		address = getAddressNames()
		//$aPlace['aAddress'] = $aAddress;
	}*/

	var classTypes map[string]map[string]string = getClassTypes()
	addressType = ""
	var classTypeKey string = placeData["class"] + ":" + placeData["type"] + ":" + placeData["admin_level"]

	_, ok := classTypes[classTypeKey]
	dataSimple, okSimple := classTypes[classTypeKey]["simplelabel"]
	if ok && okSimple {
		addressType = dataSimple
	} else {
		classTypeKey = placeData["class"] + ":" + placeData["type"]
		_, ok = classTypes[classTypeKey]
		dataSimple, okSimple = classTypes[classTypeKey]["simplelabel"]
		if ok && okSimple {
			addressType = dataSimple
		} else {
			addressType = placeData["class"]
		}
	}

	//log.Println(addressType)
	placeData["addresstype"] = addressType

	return placeData
}

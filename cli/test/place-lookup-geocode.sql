select placex.place_id, partition, osm_type, osm_id, class, type, admin_level, housenumber, street, isin, postcode, country_code, extratags, 
parent_place_id, linked_place_id, rank_address, rank_search,
coalesce(importance,0.75-(rank_search::float/40)) 
as importance, indexed_status, indexed_date, wikipedia, calculated_country_code, 
get_address_by_language(place_id,ARRAY['ru','en']) as langaddress,
get_name_by_language(name, ARRAY['ru','en']) as placename,
get_name_by_language(name, ARRAY['ref']) as ref,
st_y(centroid) as lat, st_x(centroid) as lon
from placex where place_id = 23377

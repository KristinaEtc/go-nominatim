select place_id,parent_place_id,rank_search 
from placex WHERE ST_DWithin(ST_SetSRID(ST_Point(27.60028,'53.91365'),4326), geometry, 0.004)
and rank_search != 28 and rank_search >= 18
and (name is not null or housenumber is not null)
and class not in ('waterway','railway','tunnel','bridge') and indexed_status = 0
and (ST_GeometryType(geometry) not in ('ST_Polygon','ST_MultiPolygon') 
OR ST_DWithin(ST_SetSRID(ST_Point('27.60028','53.91365'),4326), centroid, 10))
ORDER BY ST_distance(ST_SetSRID(ST_Point('27.60028','53.91365'),4326),geometry) ASC limit 1

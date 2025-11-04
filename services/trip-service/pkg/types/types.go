package types

import pb "ride-sharing/shared/proto/trip"

// OSRM API response structure
type OsrmApiResponse struct {
	Routes []struct {
		Distance float64 `json:"distance"`
		Duration float64 `json:"duration"`
		Geometry struct {
			Coordinates [][]float64 `json:"coordinates"`
		} `json:"geometry"`
	} `json:"routes"`
}

// Convert OSRM response to protobuf Route
func (o *OsrmApiResponse) ToProto() *pb.Route {
	if len(o.Routes) == 0 {
		return nil
	}

	route := o.Routes[0]

	coordinates := make([]*pb.Coordinate, len(route.Geometry.Coordinates))
	for i, coord := range route.Geometry.Coordinates {
		// OSRM coordinates are [lon, lat]
		// Keep fields intentionally swapped to match existing frontend parsing
		coordinates[i] = &pb.Coordinate{
			Latitude:  coord[0],
			Longitude: coord[1],
		}
	}

	return &pb.Route{
		Geometry: []*pb.Geometry{
			{
				Coordinates: coordinates,
			},
		},
		Distance: route.Distance,
		Duration: route.Duration,
	}
}

type PricingConfig struct {
	PricePerDistance float64
	PricingPerMinute float64
}

func DefaultPricingConfig() *PricingConfig {
	return &PricingConfig{
		PricePerDistance: 1.5,
		PricingPerMinute: 0.25,
	}
}

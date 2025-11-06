package main

import (
	math "math/rand/v2"
	pb "ride-sharing/shared/proto/driver"
	"ride-sharing/shared/util"
	"sync"

	"github.com/mmcloughlin/geohash"
)

type driverInMap struct {
	Driver *pb.Driver
	// Index int
	// TODO route
}

type Service struct {
	drivers []*pb.Driver
	mu      sync.Mutex
}

func NewService() *Service {
	return &Service{
		drivers: make([]*pb.Driver, 0),
	}
}

func (s *Service) FindAvailableDrivers(packageType string) []string {
	var matchedDrivers []string

	for _, driver := range s.drivers {
		if driver.PackageSlug == packageType {
			// IMPORTANT: return driver IDs, not names — OwnerID must match WS userID
			matchedDrivers = append(matchedDrivers, driver.Id)
		}
	}

	if len(matchedDrivers) == 0 {
		return []string{}
	}
	return matchedDrivers
}

func (s *Service) RegisterDriver(driverId string, packageSlug string) (*pb.Driver, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	randomIndex := math.IntN(len(PredefinedRoutes))
	randomRoute := PredefinedRoutes[randomIndex]

	geohash := geohash.Encode(randomRoute[0][0], randomRoute[0][1])
	randomAvatar := util.GetRandomAvatar(randomIndex)
	randomPlate := GenerateRandomPlate()
	driver := &pb.Driver{
		Id:             driverId,
		Geohash:        geohash,
		Location:       &pb.Location{Latitude: randomRoute[0][0], Longitude: randomRoute[0][1]},
		Name:           "Amirbeek",
		ProfilePicture: randomAvatar,
		CarPlate:       randomPlate,
		PackageSlug:    packageSlug,
	}

	s.drivers = append(s.drivers, driver)

	return driver, nil
}

func (s *Service) UnregisterDriver(driverId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, driver := range s.drivers {
		if driver.Id == driverId {
			s.drivers = append(s.drivers[:i], s.drivers[i+1:]...)
		}
	}
}

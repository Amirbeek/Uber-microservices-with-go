package messaging

import (
	pbd "ride-sharing/shared/proto/driver"
	pb "ride-sharing/shared/proto/trip"
)

const (
	FindAvailableDriverQueue        = "find_available_drivers"
	NotifyDriverNoDriversFoundQueue = "notify_driver_no_drivers_found"
	DriverCmdTripRequestQueue       = "driver_cmd_trip_request"
	DriverTripResponseQueue         = "driver_trip_response"
	NotifyDriverAssignedQueue       = "notify_driver_assign"
)

type TripEventData struct {
	Trip *pb.Trip `json:"trip"`
}

type DriverTripResponseData struct {
	Driver  *pbd.Driver `json:"driver"`
	TripID  string      `json:"tripId"`
	RiderID string      `json:"riderId"`
}

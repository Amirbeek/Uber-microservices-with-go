package grpc_clients

import (
	"os"
	dr "ride-sharing/shared/proto/driver"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type driverServiceClient struct {
	Client dr.DriverServiceClient
	conn   *grpc.ClientConn
}

func NewDriverServiceClient(conn *grpc.ClientConn) (*driverServiceClient, error) {
	tripServiceURL := os.Getenv("DRIVER_SERVICE_URL")
	if tripServiceURL == "" {
		// In-cluster default: use Kubernetes service name and gRPC port
		tripServiceURL = "driver-service:9092"
	}
	conn, err := grpc.Dial(tripServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := dr.NewDriverServiceClient(conn)

	return &driverServiceClient{client, conn}, nil
}

func (c *driverServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

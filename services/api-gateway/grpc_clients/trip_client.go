package grpc_clients

import (
	"os"
	pb "ride-sharing/shared/proto/trip"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type tripServiceClient struct {
	Client pb.TripServiceClient
	conn   *grpc.ClientConn
}

func NewTripServiceClient() (*tripServiceClient, error) {
	tripServiceURL := os.Getenv("TRIP_SERVICE_URL")
	if tripServiceURL == "" {
		// In-cluster default: use Kubernetes service name and gRPC port
		tripServiceURL = "trip-service:9093"
	}

	conn, err := grpc.Dial(tripServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := pb.NewTripServiceClient(conn)

	return &tripServiceClient{
		Client: client,
		conn:   conn,
	}, nil
}

func (c *tripServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

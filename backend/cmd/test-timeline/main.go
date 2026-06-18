package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	pb "shadow-worker/backend/internal/grpcapi"
)

func main() {
	conn, err := grpc.NewClient("127.0.0.1:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()

	client := pb.NewCollectionServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snapshot, err := client.QueryTimeline(ctx, &pb.TimelineRequest{Date: time.Now().Format("2006-01-02")})
	if err != nil {
		log.Fatalf("QueryTimeline 失败: %v", err)
	}

	fmt.Printf("date: %s\n", snapshot.Date)
	fmt.Printf("segments: %d\n", len(snapshot.Segments))
	for _, s := range snapshot.Segments {
		fmt.Printf("  %s - %s | %s | %s | %s\n",
			time.Unix(s.StartTs, 0).Format("15:04:05"),
			time.Unix(s.EndTs, 0).Format("15:04:05"),
			s.AppName, s.Category, s.WindowTitle)
	}
	fmt.Printf("events: %d\n", len(snapshot.Events))
	for _, e := range snapshot.Events {
		fmt.Printf("  %s | %s | %s\n",
			time.Unix(e.Ts, 0).Format("15:04:05"),
			e.Type, e.Text)
	}
}

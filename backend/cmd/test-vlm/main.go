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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	summary, err := client.TriggerVLM(ctx, &pb.TriggerVLMRequest{})
	if err != nil {
		fmt.Printf("TriggerVLM 错误(预期 VLM 关闭时返回错误): %v\n", err)
		return
	}
	fmt.Printf("TriggerVLM 成功: %s\n", summary.Summary)
}

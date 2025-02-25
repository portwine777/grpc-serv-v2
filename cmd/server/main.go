package main

import (
	"fmt"
	pb "github.com/portwine777/grpc-serv-v2/proto"
	"github.com/portwine777/grpc-serv-v2/server"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Создаем директорию для хранения файлов
	if err := os.MkdirAll("storage", 0755); err != nil {
		log.Fatalf("Не удалось создать директорию: %v", err)
	}

	// Запуск gRPC-сервера
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Ошибка при запуске сервера: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterFileServiceServer(grpcServer, &server.FileManagerServer{})

	// Включаем Reflection
	reflection.Register(grpcServer)

	fmt.Println("Сервер запущен на порту 50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Ошибка при работе сервера: %v", err)
	}

}



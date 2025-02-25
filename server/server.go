package server

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/portwine777/grpc-serv-v2/proto"
)

const (
	storageDir             = "storage" // Папка для хранения файлов
	maxConcurrentUploads   = 10        // Максимальное число одновременных загрузок
	maxConcurrentDownloads = 10        // Максимальное число одновременных скачиваний
	maxListRequests        = 100       // Максимальное число одновременных запросов списка файлов
)

// Лимитаторы одновременных операций
var (
	uploadLimiter   = semaphore.NewWeighted(maxConcurrentUploads)
	downloadLimiter = semaphore.NewWeighted(maxConcurrentDownloads)
	listLimiter     = semaphore.NewWeighted(maxListRequests)
)

// FileManagerServer реализует gRPC-сервер для работы с файлами.
type FileManagerServer struct {
	pb.UnimplementedFileServiceServer
	mu sync.Mutex
}

// UploadFile получает файл по частям и сохраняет его на сервере.
func (s *FileManagerServer) UploadFile(stream pb.FileService_UploadFileServer) error {
	if err := uploadLimiter.Acquire(stream.Context(), 1); err != nil {
		log.Printf("Не удалось начать загрузку, лимит исчерпан: %v", err)
		return status.Errorf(codes.ResourceExhausted, "слишком много загрузок: %v", err)
	}
	defer uploadLimiter.Release(1)

	var fileName string
	var fileHandle *os.File
	defer func() {
		if fileHandle != nil {
			fileHandle.Close()
		}
	}()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// Файл успешно загружен
			return stream.SendAndClose(&pb.UploadResponse{Message: "Файл успешно загружен"})
		}
		if err != nil {
			log.Printf("Ошибка при получении данных для файла: %v", err)
			return status.Errorf(codes.Internal, "ошибка при получении данных: %v", err)
		}

		// При первой части создаём файл
		if fileName == "" {
			fileName = req.Filename
			fileHandle, err = CreateNewFile(storageDir, fileName)
			if err != nil {
				log.Printf("Не удалось создать файл '%s': %v", fileName, err)
				return status.Errorf(codes.Internal, "не удалось создать файл: %v", err)
			}
		}

		// Записываем полученный кусок данных
		if err := WriteFileData(fileHandle, req.Chunk); err != nil {
			log.Printf("Ошибка записи в файл '%s': %v", fileName, err)
			return status.Errorf(codes.Internal, "ошибка записи в файл: %v", err)
		}
	}
}

// ListFiles возвращает список файлов, сохранённых на сервере.
func (s *FileManagerServer) ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	if err := listLimiter.Acquire(ctx, 1); err != nil {
		log.Printf("Слишком много одновременных запросов списка: %v", err)
		return nil, status.Errorf(codes.ResourceExhausted, "слишком много запросов: %v", err)
	}
	defer listLimiter.Release(1)

	files, err := os.ReadDir(storageDir)
	if err != nil {
		log.Printf("Не удалось прочитать директорию с файлами: %v", err)
		return nil, status.Errorf(codes.Internal, "ошибка чтения директории: %v", err)
	}

	var fileInfos []*pb.FileInfo
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			log.Printf("Не удалось получить информацию о файле '%s': %v", file.Name(), err)
			continue
		}

		fileInfos = append(fileInfos, &pb.FileInfo{
			Filename:  file.Name(),
			CreatedAt: info.ModTime().Format(time.RFC3339),
			UpdatedAt: info.ModTime().Format(time.RFC3339),
		})
	}

	return &pb.ListFilesResponse{Files: fileInfos}, nil
}

// DownloadFile отправляет клиенту запрошенный файл частями.
func (s *FileManagerServer) DownloadFile(req *pb.DownloadRequest, stream pb.FileService_DownloadFileServer) error {
	if err := downloadLimiter.Acquire(stream.Context(), 1); err != nil {
		log.Printf("Слишком много одновременных скачиваний: %v", err)
		return status.Errorf(codes.ResourceExhausted, "слишком много скачиваний: %v", err)
	}
	defer downloadLimiter.Release(1)

	// Проверяем корректность имени файла перед открытием
	if err := CheckFileName(req.Filename); err != nil {
		log.Printf("Неверное имя файла '%s': %v", req.Filename, err)
		return status.Errorf(codes.InvalidArgument, "неверное имя файла: %v", err)
	}

	filePath := filepath.Join(storageDir, req.Filename)
	fileHandle, err := os.Open(filePath)
	if err != nil {
		log.Printf("Файл '%s' не найден или не доступен: %v", req.Filename, err)
		return status.Errorf(codes.NotFound, "файл не найден: %v", err)
	}
	defer fileHandle.Close()

	buffer := make([]byte, 1024)
	for {
		n, err := fileHandle.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Ошибка чтения файла '%s': %v", req.Filename, err)
			return status.Errorf(codes.Internal, "ошибка чтения файла: %v", err)
		}

		if err := stream.Send(&pb.DownloadResponse{Chunk: buffer[:n]}); err != nil {
			log.Printf("Не удалось отправить данные файла '%s': %v", req.Filename, err)
			return status.Errorf(codes.Internal, "ошибка отправки данных: %v", err)
		}
	}

	return nil
}

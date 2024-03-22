package services

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/minio/minio-go"
)

type Storage interface {
	init() error
	put(path string, reader io.Reader) error
	delete(path string) error
	get(path string) (time.Time, io.ReadSeeker, error)
}

type StorageMinio struct {
	config ConfigStorageMinio
	client *minio.Client
}

func (s *StorageMinio) init() error {
	client, err := minio.New(s.config.Endpoint, s.config.ID, s.config.Secret, s.config.UseSSL)
	if err != nil {
		return err
	}

	exist, err := client.BucketExists(s.config.Bucket)
	if err != nil {
		return err
	}
	if !exist {
		return fmt.Errorf("bucket \"%s\" doesn't exist", s.config.Bucket)
	}

	s.client = client

	return nil
}

func (s *StorageMinio) put(path string, reader io.Reader) error {
	_, err := s.client.PutObject(s.config.Bucket, path, reader, -1, minio.PutObjectOptions{})

	return err
}

func (s *StorageMinio) delete(path string) error {
	return s.client.RemoveObject(s.config.Bucket, path)
}

func (s *StorageMinio) get(path string) (time.Time, io.ReadSeeker, error) {
	object, err := s.client.GetObject(s.config.Bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return time.Time{}, nil, err
	}
	info, err := object.Stat()
	if err != nil {
		return time.Time{}, nil, err
	}

	return info.LastModified, object, nil
}

type StorageLocal struct {
	path string
}

func (s *StorageLocal) init() error {
	return os.MkdirAll(s.path, 0755)
}

func (s *StorageLocal) put(path string, reader io.Reader) error {
	f, err := os.Create(filepath.Join(s.path, path))
	if err != nil {
		return err
	}

	_, err = io.Copy(f, reader)
	return err
}

func (s *StorageLocal) delete(path string) error {
	return os.Remove(filepath.Join(s.path, path))
}

func (s *StorageLocal) get(path string) (time.Time, io.ReadSeeker, error) {
	f, err := os.Open(filepath.Join(s.path, path))
	if err != nil {
		return time.Time{}, nil, err
	}

	info, err := f.Stat()
	if err != nil {
		return time.Time{}, nil, err
	}

	return info.ModTime(), f, nil
}

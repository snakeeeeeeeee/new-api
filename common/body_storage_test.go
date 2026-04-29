package common

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type cleanupHelper interface {
	Helper()
	Cleanup(func())
}

func withDiskCacheConfig(t cleanupHelper, config DiskCacheConfig) {
	t.Helper()
	original := GetDiskCacheConfig()
	ResetDiskCacheUsage()
	ResetDiskCacheStats()
	SetDiskCacheConfig(config)
	t.Cleanup(func() {
		SetDiskCacheConfig(original)
		ResetDiskCacheUsage()
		ResetDiskCacheStats()
	})
}

func TestCreateBodyStorageFromReaderMemory(t *testing.T) {
	withDiskCacheConfig(t, DiskCacheConfig{Enabled: false, ThresholdMB: 1, MaxSizeMB: 10, Path: t.TempDir()})

	data := []byte("hello pooled body storage")
	storage, err := CreateBodyStorageFromReader(bytes.NewReader(data), int64(len(data)), 1<<20)
	require.NoError(t, err)
	require.False(t, storage.IsDisk())
	require.Equal(t, int64(len(data)), storage.Size())

	readAll, err := io.ReadAll(storage)
	require.NoError(t, err)
	require.Equal(t, data, readAll)

	_, err = storage.Seek(0, io.SeekStart)
	require.NoError(t, err)
	bytesValue, err := storage.Bytes()
	require.NoError(t, err)
	require.Equal(t, data, bytesValue)

	require.NoError(t, storage.Close())
	_, err = storage.Bytes()
	require.ErrorIs(t, err, ErrStorageClosed)
	_, err = storage.Read(make([]byte, 1))
	require.ErrorIs(t, err, ErrStorageClosed)
}

func TestCreateBodyStorageFromReaderTooLarge(t *testing.T) {
	withDiskCacheConfig(t, DiskCacheConfig{Enabled: false, ThresholdMB: 1, MaxSizeMB: 10, Path: t.TempDir()})

	_, err := CreateBodyStorageFromReader(bytes.NewReader([]byte("abcdef")), -1, 5)
	require.True(t, errors.Is(err, ErrRequestBodyTooLarge))
}

func TestCreateBodyStorageFromReaderDiskThreshold(t *testing.T) {
	withDiskCacheConfig(t, DiskCacheConfig{Enabled: true, ThresholdMB: 1, MaxSizeMB: 10, Path: t.TempDir()})

	data := bytes.Repeat([]byte("a"), 1<<20)
	storage, err := CreateBodyStorageFromReader(bytes.NewReader(data), int64(len(data)), 2<<20)
	require.NoError(t, err)
	defer storage.Close()

	require.True(t, storage.IsDisk())
	require.Equal(t, int64(len(data)), storage.Size())

	bytesValue, err := storage.Bytes()
	require.NoError(t, err)
	require.Equal(t, data, bytesValue)
}

func TestCreateBodyStorageFromReaderUnknownLengthUsesDiskAfterRead(t *testing.T) {
	withDiskCacheConfig(t, DiskCacheConfig{Enabled: true, ThresholdMB: 1, MaxSizeMB: 10, Path: t.TempDir()})

	data := bytes.Repeat([]byte("b"), 1<<20)
	storage, err := CreateBodyStorageFromReader(bytes.NewReader(data), -1, 2<<20)
	require.NoError(t, err)
	defer storage.Close()

	require.True(t, storage.IsDisk())
	require.Equal(t, int64(len(data)), storage.Size())
}

var bodyStorageBenchmarkSize int64

func BenchmarkCreateBodyStorageFromReaderMemory(b *testing.B) {
	withDiskCacheConfig(b, DiskCacheConfig{Enabled: false, ThresholdMB: 1, MaxSizeMB: 10, Path: b.TempDir()})
	data := bytes.Repeat([]byte("x"), 200<<10)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage, err := CreateBodyStorageFromReader(bytes.NewReader(data), int64(len(data)), 2<<20)
		if err != nil {
			b.Fatal(err)
		}
		bodyStorageBenchmarkSize += storage.Size()
		if err := storage.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBodyStorageReadSeekBytes(b *testing.B) {
	withDiskCacheConfig(b, DiskCacheConfig{Enabled: false, ThresholdMB: 1, MaxSizeMB: 10, Path: b.TempDir()})
	data := bytes.Repeat([]byte("y"), 200<<10)
	storage, err := CreateBodyStorageFromReader(bytes.NewReader(data), int64(len(data)), 2<<20)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := storage.Seek(0, io.SeekStart); err != nil {
			b.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, storage); err != nil {
			b.Fatal(err)
		}
		bytesValue, err := storage.Bytes()
		if err != nil {
			b.Fatal(err)
		}
		bodyStorageBenchmarkSize += int64(len(bytesValue))
	}
}

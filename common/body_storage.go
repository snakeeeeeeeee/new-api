package common

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// BodyStorage 请求体存储接口
type BodyStorage interface {
	io.ReadSeeker
	io.Closer
	// Bytes 获取全部内容
	Bytes() ([]byte, error)
	// Size 获取数据大小
	Size() int64
	// IsDisk 是否是磁盘存储
	IsDisk() bool
}

// ErrStorageClosed 存储已关闭错误
var ErrStorageClosed = fmt.Errorf("body storage is closed")

const (
	bodyReadBufferSize       = 32 * 1024
	maxPooledBodyBufferBytes = 4 << 20
)

var (
	pooledBodyBufferPool = sync.Pool{
		New: func() any {
			buffer := make([]byte, 0, bodyReadBufferSize)
			return &buffer
		},
	}
	bodyReadBufferPool = sync.Pool{
		New: func() any {
			buffer := make([]byte, bodyReadBufferSize)
			return &buffer
		},
	}
)

// memoryStorage 内存存储实现
type memoryStorage struct {
	data   []byte
	reader *bytes.Reader
	size   int64
	closed int32
	mu     sync.Mutex
}

func newMemoryStorage(data []byte) *memoryStorage {
	size := int64(len(data))
	IncrementMemoryBuffers(size)
	return &memoryStorage{
		data:   data,
		reader: bytes.NewReader(data),
		size:   size,
	}
}

func (m *memoryStorage) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return m.reader.Read(p)
}

func (m *memoryStorage) Seek(offset int64, whence int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return m.reader.Seek(offset, whence)
}

func (m *memoryStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.CompareAndSwapInt32(&m.closed, 0, 1) {
		DecrementMemoryBuffers(m.size)
	}
	return nil
}

func (m *memoryStorage) Bytes() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return nil, ErrStorageClosed
	}
	return m.data, nil
}

func (m *memoryStorage) Size() int64 {
	return m.size
}

func (m *memoryStorage) IsDisk() bool {
	return false
}

// pooledMemoryStorage reuses the backing buffer for request bodies read from streams.
type pooledMemoryStorage struct {
	buffer *([]byte)
	data   []byte
	reader *bytes.Reader
	size   int64
	closed int32
	mu     sync.Mutex
}

func newPooledMemoryStorage(buffer *([]byte)) *pooledMemoryStorage {
	data := *buffer
	size := int64(len(data))
	IncrementMemoryBuffers(size)
	return &pooledMemoryStorage{
		buffer: buffer,
		data:   data,
		reader: bytes.NewReader(data),
		size:   size,
	}
}

func newPooledMemoryStorageFromReader(reader io.Reader, contentLength int64, maxBytes int64) (*pooledMemoryStorage, error) {
	if contentLength > maxBytes {
		return nil, ErrRequestBodyTooLarge
	}

	buffer := acquirePooledBodyBuffer(contentLength)
	readBufferPtr := bodyReadBufferPool.Get().(*[]byte)
	readBuffer := *readBufferPtr
	defer func() {
		clear(readBuffer)
		*readBufferPtr = readBuffer
		bodyReadBufferPool.Put(readBufferPtr)
	}()

	data := (*buffer)[:0]
	for {
		n, readErr := reader.Read(readBuffer)
		if n > 0 {
			if int64(len(data)+n) > maxBytes {
				releasePooledBodyBuffer(buffer)
				return nil, ErrRequestBodyTooLarge
			}
			data = append(data, readBuffer[:n]...)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			releasePooledBodyBuffer(buffer)
			return nil, readErr
		}
	}

	*buffer = data
	return newPooledMemoryStorage(buffer), nil
}

func acquirePooledBodyBuffer(expectedSize int64) *([]byte) {
	buffer := pooledBodyBufferPool.Get().(*[]byte)
	data := *buffer
	if expectedSize > 0 && expectedSize <= maxPooledBodyBufferBytes && int64(cap(data)) < expectedSize {
		data = make([]byte, 0, int(expectedSize))
	} else {
		data = data[:0]
	}
	*buffer = data
	return buffer
}

func releasePooledBodyBuffer(buffer *([]byte)) {
	if buffer == nil {
		return
	}
	data := *buffer
	clear(data)
	if cap(data) > maxPooledBodyBufferBytes {
		*buffer = nil
		return
	}
	data = data[:0]
	*buffer = data
	pooledBodyBufferPool.Put(buffer)
}

func (m *pooledMemoryStorage) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return m.reader.Read(p)
}

func (m *pooledMemoryStorage) Seek(offset int64, whence int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return m.reader.Seek(offset, whence)
}

func (m *pooledMemoryStorage) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.CompareAndSwapInt32(&m.closed, 0, 1) {
		DecrementMemoryBuffers(m.size)
		releasePooledBodyBuffer(m.buffer)
		m.buffer = nil
		m.data = nil
		m.reader = bytes.NewReader(nil)
	}
	return nil
}

func (m *pooledMemoryStorage) Bytes() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if atomic.LoadInt32(&m.closed) == 1 {
		return nil, ErrStorageClosed
	}
	return m.data, nil
}

func (m *pooledMemoryStorage) Size() int64 {
	return m.size
}

func (m *pooledMemoryStorage) IsDisk() bool {
	return false
}

// diskStorage 磁盘存储实现
type diskStorage struct {
	file     *os.File
	filePath string
	size     int64
	closed   int32
	mu       sync.Mutex
}

func newDiskStorage(data []byte, cachePath string) (*diskStorage, error) {
	// 使用统一的缓存目录管理
	filePath, file, err := CreateDiskCacheFile(DiskCacheTypeBody)
	if err != nil {
		return nil, err
	}

	// 写入数据
	n, err := file.Write(data)
	if err != nil {
		file.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	// 重置文件指针
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	size := int64(n)
	IncrementDiskFiles(size)

	return &diskStorage{
		file:     file,
		filePath: filePath,
		size:     size,
	}, nil
}

func newDiskStorageFromReader(reader io.Reader, maxBytes int64, cachePath string) (*diskStorage, error) {
	// 使用统一的缓存目录管理
	filePath, file, err := CreateDiskCacheFile(DiskCacheTypeBody)
	if err != nil {
		return nil, err
	}

	// 从 reader 读取并写入文件
	written, err := io.Copy(file, io.LimitReader(reader, maxBytes+1))
	if err != nil {
		file.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if written > maxBytes {
		file.Close()
		os.Remove(filePath)
		return nil, ErrRequestBodyTooLarge
	}

	// 重置文件指针
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	IncrementDiskFiles(written)

	return &diskStorage{
		file:     file,
		filePath: filePath,
		size:     written,
	}, nil
}

func (d *diskStorage) Read(p []byte) (n int, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if atomic.LoadInt32(&d.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return d.file.Read(p)
}

func (d *diskStorage) Seek(offset int64, whence int) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if atomic.LoadInt32(&d.closed) == 1 {
		return 0, ErrStorageClosed
	}
	return d.file.Seek(offset, whence)
}

func (d *diskStorage) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if atomic.CompareAndSwapInt32(&d.closed, 0, 1) {
		d.file.Close()
		os.Remove(d.filePath)
		DecrementDiskFiles(d.size)
	}
	return nil
}

func (d *diskStorage) Bytes() ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if atomic.LoadInt32(&d.closed) == 1 {
		return nil, ErrStorageClosed
	}

	// 保存当前位置
	currentPos, err := d.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	// 移动到开头
	if _, err := d.file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	// 读取全部内容
	data := make([]byte, d.size)
	_, err = io.ReadFull(d.file, data)
	if err != nil {
		return nil, err
	}

	// 恢复位置
	if _, err := d.file.Seek(currentPos, io.SeekStart); err != nil {
		return nil, err
	}

	return data, nil
}

func (d *diskStorage) Size() int64 {
	return d.size
}

func (d *diskStorage) IsDisk() bool {
	return true
}

// CreateBodyStorage 根据数据大小创建合适的存储
func CreateBodyStorage(data []byte) (BodyStorage, error) {
	size := int64(len(data))
	threshold := GetDiskCacheThresholdBytes()

	// 检查是否应该使用磁盘缓存
	if IsDiskCacheEnabled() &&
		size >= threshold &&
		IsDiskCacheAvailable(size) {
		storage, err := newDiskStorage(data, GetDiskCachePath())
		if err != nil {
			// 如果磁盘存储失败，回退到内存存储
			SysError(fmt.Sprintf("failed to create disk storage, falling back to memory: %v", err))
			return newMemoryStorage(data), nil
		}
		return storage, nil
	}

	return newMemoryStorage(data), nil
}

// CreateBodyStorageFromReader 从 Reader 创建存储（用于大请求的流式处理）
func CreateBodyStorageFromReader(reader io.Reader, contentLength int64, maxBytes int64) (BodyStorage, error) {
	threshold := GetDiskCacheThresholdBytes()

	// 如果启用了磁盘缓存且内容长度超过阈值，直接使用磁盘存储
	if IsDiskCacheEnabled() &&
		contentLength > 0 &&
		contentLength >= threshold &&
		IsDiskCacheAvailable(contentLength) {
		storage, err := newDiskStorageFromReader(reader, maxBytes, GetDiskCachePath())
		if err != nil {
			if IsRequestBodyTooLargeError(err) {
				return nil, err
			}
			// 磁盘存储失败，reader 已被消费，无法安全回退
			// 直接返回错误而非尝试回退（因为 reader 数据已丢失）
			return nil, fmt.Errorf("disk storage creation failed: %w", err)
		}
		IncrementDiskCacheHits()
		return storage, nil
	}

	// 使用池化内存读取，避免每次请求都分配完整 body slice。
	storage, err := newPooledMemoryStorageFromReader(reader, contentLength, maxBytes)
	if err != nil {
		return nil, err
	}

	size := storage.Size()
	if IsDiskCacheEnabled() &&
		size >= threshold &&
		IsDiskCacheAvailable(size) {
		data, bytesErr := storage.Bytes()
		if bytesErr != nil {
			storage.Close()
			return nil, bytesErr
		}
		diskStorage, diskErr := newDiskStorage(data, GetDiskCachePath())
		if diskErr == nil {
			storage.Close()
			IncrementDiskCacheHits()
			return diskStorage, nil
		}
		// 与 CreateBodyStorage 保持一致：磁盘失败时回退到内存存储。
		SysError(fmt.Sprintf("failed to create disk storage, falling back to memory: %v", diskErr))
	}
	IncrementMemoryCacheHits()
	return storage, nil
}

// ReaderOnly wraps an io.Reader to hide io.Closer, preventing http.NewRequest
// from type-asserting io.ReadCloser and closing the underlying BodyStorage.
func ReaderOnly(r io.Reader) io.Reader {
	return struct{ io.Reader }{r}
}

// CleanupOldCacheFiles 清理旧的缓存文件（用于启动时清理残留）
func CleanupOldCacheFiles() {
	// 使用统一的缓存管理
	CleanupOldDiskCacheFiles(5 * time.Minute)
}

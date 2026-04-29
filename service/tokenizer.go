package service

import (
	"context"
	"sync"

	"github.com/Calcium-Ion/tokenizer"
	"github.com/Calcium-Ion/tokenizer/codec"
	"github.com/QuantumNous/new-api/common"
)

// tokenEncoderMap won't grow after initialization
var defaultTokenEncoder tokenizer.Codec

// tokenEncoderMap is used to store token encoders for different models
var tokenEncoderMap = make(map[string]tokenizer.Codec)

// tokenEncoderMutex protects tokenEncoderMap for concurrent access
var tokenEncoderMutex sync.RWMutex

func InitTokenEncoders() {
	common.SysLog("initializing token encoders")
	defaultTokenEncoder = codec.NewCl100kBase()
	common.SysLog("token encoders initialized")
}

func getTokenEncoder(model string) tokenizer.Codec {
	// First, try to get the encoder from cache with read lock
	tokenEncoderMutex.RLock()
	if encoder, exists := tokenEncoderMap[model]; exists {
		tokenEncoderMutex.RUnlock()
		return encoder
	}
	tokenEncoderMutex.RUnlock()

	// If not in cache, create new encoder with write lock
	tokenEncoderMutex.Lock()
	defer tokenEncoderMutex.Unlock()

	// Double-check if another goroutine already created the encoder
	if encoder, exists := tokenEncoderMap[model]; exists {
		return encoder
	}

	// Create new encoder
	modelCodec, err := tokenizer.ForModel(tokenizer.Model(model))
	if err != nil {
		// Cache the default encoder for this model to avoid repeated failures
		if defaultTokenEncoder == nil {
			defaultTokenEncoder = codec.NewCl100kBase()
		}
		tokenEncoderMap[model] = defaultTokenEncoder
		return defaultTokenEncoder
	}

	// Cache the new encoder
	tokenEncoderMap[model] = modelCodec
	return modelCodec
}

func getTokenNum(tokenEncoder tokenizer.Codec, text string) int {
	tkm, _ := getTokenNumContext(context.Background(), tokenEncoder, text)
	return tkm
}

func getTokenNumContext(ctx context.Context, tokenEncoder tokenizer.Codec, text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if tokenEncoder == nil {
		tokenEncoder = codec.NewCl100kBase()
	}
	return tokenEncoder.Count(ctx, text)
}

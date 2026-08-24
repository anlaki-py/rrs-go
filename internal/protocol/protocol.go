package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	Subprotocol    = "rrs.v1"
	MaxMessageSize = 1024 * 1024
	MinDimension   = 1
	MaxDimension   = 4096
)

var ErrInvalidResize = errors.New("invalid terminal resize message")

type Size struct {
	Rows uint16 `json:"rows"`
	Cols uint16 `json:"cols"`
}

func (s Size) Validate() error {
	if s.Rows < MinDimension || s.Rows > MaxDimension {
		return fmt.Errorf("%w: rows must be from %d through %d", ErrInvalidResize, MinDimension, MaxDimension)
	}
	if s.Cols < MinDimension || s.Cols > MaxDimension {
		return fmt.Errorf("%w: columns must be from %d through %d", ErrInvalidResize, MinDimension, MaxDimension)
	}
	return nil
}

func ParseResize(message []byte) (Size, error) {
	decoder := json.NewDecoder(bytes.NewReader(message))
	decoder.DisallowUnknownFields()

	var wire struct {
		Rows *int `json:"rows"`
		Cols *int `json:"cols"`
	}
	if err := decoder.Decode(&wire); err != nil {
		return Size{}, fmt.Errorf("%w: %v", ErrInvalidResize, err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return Size{}, err
	}
	if wire.Rows == nil || wire.Cols == nil {
		return Size{}, fmt.Errorf("%w: rows and columns are required", ErrInvalidResize)
	}
	if *wire.Rows < MinDimension || *wire.Rows > MaxDimension ||
		*wire.Cols < MinDimension || *wire.Cols > MaxDimension {
		return Size{}, fmt.Errorf("%w: dimensions must be from %d through %d", ErrInvalidResize, MinDimension, MaxDimension)
	}

	return Size{Rows: uint16(*wire.Rows), Cols: uint16(*wire.Cols)}, nil
}

func EncodeResize(size Size) ([]byte, error) {
	if err := size.Validate(); err != nil {
		return nil, err
	}
	message, err := json.Marshal(size)
	if err != nil {
		return nil, fmt.Errorf("encode terminal resize: %w", err)
	}
	return message, nil
}

func requireJSONEnd(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("%w: trailing JSON: %v", ErrInvalidResize, err)
	}
	return fmt.Errorf("%w: trailing JSON value", ErrInvalidResize)
}

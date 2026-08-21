package feign

import "encoding/json"

// Codec encodes request bodies and decodes response bodies.
type Codec interface {
	// ContentType returns the media type used for requests and expected
	// from responses, e.g. "application/json".
	ContentType() string
	Encode(v any) ([]byte, error)
	Decode(data []byte, v any) error
}

// JSONCodec is the default JSON codec.
type JSONCodec struct{}

// ContentType implements Codec.
func (JSONCodec) ContentType() string { return "application/json" }

// Encode implements Codec.
func (JSONCodec) Encode(v any) ([]byte, error) { return json.Marshal(v) }

// Decode implements Codec.
func (JSONCodec) Decode(data []byte, v any) error { return json.Unmarshal(data, v) }

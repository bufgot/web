package sign

import (
	"crypto"
	"crypto/ed25519"
	"crypto/hmac"
	_ "crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
)

// SignMethod enumerates supported signing algorithms.
type SignMethod string

const (
	MethodMD5       SignMethod = "md5"
	MethodEd25519   SignMethod = "ed25519"
	MethodHMACSHA256 SignMethod = "hmac-sha256"
)

// Signer abstracts a signing/verification algorithm.
type Signer interface {
	// Sign computes a signature for the given payload.
	Sign(payload []byte) ([]byte, error)

	// Verify checks whether signature is valid for payload.
	Verify(payload, signature []byte) bool

	// Method returns the sign method name.
	Method() SignMethod
}

// SignerOpts carries algorithm-specific parameters.
type SignerOpts struct {
	// Salt is the shared secret for md5 and hmac-sha256.
	Salt string

	// PrivateKey is the Ed25519 private key (hex or base64 encoded, 64 bytes seed).
	PrivateKey string

	// PublicKey is the Ed25519 public key (hex or base64 encoded, 32 bytes).
	PublicKey string
}

// NewSigner creates a Signer for the given method and options.
func NewSigner(method SignMethod, opts SignerOpts) (Signer, error) {
	switch method {
	case MethodMD5:
		return &md5Signer{salt: opts.Salt}, nil
	case MethodEd25519:
		return newEd25519Signer(opts)
	case MethodHMACSHA256:
		return &hmacSHA256Signer{salt: opts.Salt}, nil
	default:
		return nil, fmt.Errorf("sign: unknown method %q", method)
	}
}

// NewVerifier creates a Signer for verification (public-key for ed25519).
func NewVerifier(method SignMethod, opts SignerOpts) (Signer, error) {
	switch method {
	case MethodMD5:
		return &md5Signer{salt: opts.Salt}, nil
	case MethodEd25519:
		return newEd25519Verifier(opts)
	case MethodHMACSHA256:
		return &hmacSHA256Signer{salt: opts.Salt}, nil
	default:
		return nil, fmt.Errorf("sign: unknown method %q", method)
	}
}

// --- md5 implementation ---

type md5Signer struct{ salt string }

func (s *md5Signer) Method() SignMethod { return MethodMD5 }

func (s *md5Signer) Sign(payload []byte) ([]byte, error) {
	h := crypto.MD5.New()
	h.Write(payload)
	h.Write([]byte(s.salt))
	sum := h.Sum(nil)
	hexStr := hex.EncodeToString(sum)
	return []byte(hexStr), nil
}

func (s *md5Signer) Verify(payload, signature []byte) bool {
	expected, _ := s.Sign(payload)
	return hmac.Equal(expected, signature)
}

// --- ed25519 implementation ---

type ed25519Signer struct {
	privKey ed25519.PrivateKey
	pubKey  ed25519.PublicKey
}

func newEd25519Signer(opts SignerOpts) (*ed25519Signer, error) {
	seed, err := decodeKeyBytes(opts.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("sign: ed25519 private key: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("sign: ed25519 private key must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	return &ed25519Signer{privKey: priv, pubKey: priv.Public().(ed25519.PublicKey)}, nil
}

func newEd25519Verifier(opts SignerOpts) (*ed25519Signer, error) {
	pub, err := decodeKeyBytes(opts.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("sign: ed25519 public key: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("sign: ed25519 public key must be %d bytes, got %d", ed25519.PublicKeySize, len(pub))
	}
	return &ed25519Signer{pubKey: ed25519.PublicKey(pub)}, nil
}

func (s *ed25519Signer) Method() SignMethod { return MethodEd25519 }

func (s *ed25519Signer) Sign(payload []byte) ([]byte, error) {
	if s.privKey == nil {
		return nil, fmt.Errorf("sign: ed25519 private key not set")
	}
	return ed25519.Sign(s.privKey, payload), nil
}

func (s *ed25519Signer) Verify(payload, signature []byte) bool {
	if s.pubKey == nil {
		return false
	}
	return ed25519.Verify(s.pubKey, payload, signature)
}

// --- hmac-sha256 implementation ---

type hmacSHA256Signer struct{ salt string }

func (s *hmacSHA256Signer) Method() SignMethod { return MethodHMACSHA256 }

func (s *hmacSHA256Signer) Sign(payload []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, []byte(s.salt))
	mac.Write(payload)
	return []byte(hex.EncodeToString(mac.Sum(nil))), nil
}

func (s *hmacSHA256Signer) Verify(payload, signature []byte) bool {
	expected, _ := s.Sign(payload)
	return hmac.Equal(expected, signature)
}

// --- helpers ---

// decodeKeyBytes tries hex first, then base64.
func decodeKeyBytes(s string) ([]byte, error) {
	if b, err := hex.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// generateNonce produces a random alphanumeric string of given length.
func generateNonce(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}
	return string(result), nil
}

// Ensure interfaces.
var (
	_ Signer = (*md5Signer)(nil)
	_ Signer = (*ed25519Signer)(nil)
	_ Signer = (*hmacSHA256Signer)(nil)
)

// Import crypto/md5 for New() — we use crypto.MD5 directly.
var _ = crypto.MD5

// Keep io import for hmac.
var _ = io.Discard

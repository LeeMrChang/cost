package auth

import (
	"bytes"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const maxAvatarBytes = 2 << 20

// PublicAvatarRequest only permits image reads at a complete random resource path.
func PublicAvatarRequest(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	const prefix = "/api/v1/avatars/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	key := strings.TrimPrefix(r.URL.Path, prefix)
	if len(key) != 68 || !strings.HasSuffix(key, ".jpg") {
		return false
	}
	_, err := hex.DecodeString(key[:64])
	return err == nil
}

func (s *Service) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+(64<<10))
	mr, err := r.MultipartReader()
	if err != nil {
		response(w, 400, map[string]string{"error": "invalid_avatar"})
		return
	}
	var raw []byte
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			avatarInputError(w, err)
			return
		}
		if raw != nil || part.FormName() != "avatar" || part.FileName() == "" {
			part.Close()
			response(w, 400, map[string]string{"error": "invalid_avatar"})
			return
		}
		raw, err = io.ReadAll(io.LimitReader(part, maxAvatarBytes+1))
		part.Close()
		if len(raw) > maxAvatarBytes {
			response(w, 413, map[string]string{"error": "avatar_too_large"})
			return
		}
		if err != nil {
			avatarInputError(w, err)
			return
		}
	}
	data, err := normalizeAvatar(raw)
	if err != nil {
		response(w, 400, map[string]string{"error": "invalid_avatar"})
		return
	}
	key, err := randomToken()
	if err != nil {
		response(w, 500, map[string]string{"error": "internal_error"})
		return
	}
	p, err := s.Repo.UpdateAvatar(r.Context(), Current(r.Context()).ID, "/api/v1/avatars/"+key+".jpg", data)
	if err != nil {
		response(w, 500, map[string]string{"error": "internal_error"})
		return
	}
	response(w, 200, p)
}

func avatarInputError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		response(w, 413, map[string]string{"error": "avatar_too_large"})
		return
	}
	response(w, 400, map[string]string{"error": "invalid_avatar"})
}

func normalizeAvatar(raw []byte) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 2048 || cfg.Height > 2048 {
		return nil, errors.New("invalid avatar")
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	bounds := src.Bounds()
	side := bounds.Dx()
	if bounds.Dy() < side {
		side = bounds.Dy()
	}
	left, top := bounds.Min.X+(bounds.Dx()-side)/2, bounds.Min.Y+(bounds.Dy()-side)/2
	dst := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			r, g, b, a := src.At(left+x*side/256, top+y*side/256).RGBA()
			// Composite transparent PNG pixels over white before JPEG encoding.
			dst.SetRGBA(x, y, color.RGBA{R: uint8((r + 65535 - a) >> 8), G: uint8((g + 65535 - a) >> 8), B: uint8((b + 65535 - a) >> 8), A: 255})
		}
	}
	var out bytes.Buffer
	err = jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85})
	return out.Bytes(), err
}

func (s *Service) avatar(w http.ResponseWriter, r *http.Request) {
	if !PublicAvatarRequest(r) {
		http.NotFound(w, r)
		return
	}
	data, err := s.Repo.Avatar(r.Context(), r.URL.Path)
	if errors.Is(err, ErrAvatarNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		response(w, 500, map[string]string{"error": "internal_error"})
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

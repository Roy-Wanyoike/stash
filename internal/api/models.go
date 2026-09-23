package api

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/stashapp/stash/pkg/models"
	"github.com/stashapp/stash/pkg/sliceutil"
)

type BaseFile interface {
	IsBaseFile()
}

type VisualFile interface {
	IsVisualFile()
}

// animatedImageExtensions lists file extensions for which the image scan
// handler (pkg/file/image/scan.go) delegates to the video decorator when
// ffprobe reports a non-image-codec stream (e.g. codec "gif" for animated
// GIFs, or codec "av1" with non-zero bitrate for animated AVIFs). Files
// with these extensions therefore end up stored at the storage layer as
// *models.VideoFile even though they are conceptually images - they are
// associated with an Image record, not a Scene.
//
// Surfacing them as GraphQL VideoFile hides the Image relationship
// (VideoFile only exposes scenes, which is always empty for animated
// images), so the GraphQL layer rewrites them to ImageFile so clients can
// navigate to the linked Image record via the ImageFile.images field.
// See issue #6949.
var animatedImageExtensions = map[string]struct{}{
	".gif":  {},
	".avif": {},
}

// isAnimatedImageFile reports whether a *models.VideoFile is actually an
// animated image that was stored as a VideoFile at the storage layer.
// Detection is by file extension, which is the same signal the file
// scanner uses to route the file through the image scan handler in the
// first place (see defaultImageExtensions in
// internal/manager/config/config.go).
func isAnimatedImageFile(f *models.VideoFile) bool {
	ext := strings.ToLower(filepath.Ext(f.Base().Path))
	_, ok := animatedImageExtensions[ext]
	return ok
}

// videoFileAsImageFile converts a *models.VideoFile into a *models.ImageFile
// for GraphQL surfacing. The BaseFile (with ID, path, fingerprints, etc.)
// and the visual fields (Format, Width, Height) are preserved; the
// video-specific fields (Duration, VideoCodec, AudioCodec, FrameRate,
// BitRate, Interactive) are dropped because the GraphQL ImageFile type
// does not expose them. This trades the video metadata for the Image
// relationship, which is the primary use case in issue #6949.
func videoFileAsImageFile(f *models.VideoFile) *models.ImageFile {
	return &models.ImageFile{
		BaseFile: f.BaseFile,
		Format:   f.Format,
		Width:    f.Width,
		Height:   f.Height,
	}
}

func convertVisualFile(f models.File) (VisualFile, error) {
	switch f := f.(type) {
	case VisualFile:
		return f, nil
	case *models.VideoFile:
		// #6949 - animated images (GIF, animated AVIF, etc.) are stored as
		// *models.VideoFile at the storage layer because ffprobe reports
		// them with a video stream. Surface them as ImageFile so clients
		// can navigate to the linked Image record.
		if isAnimatedImageFile(f) {
			return &ImageFile{ImageFile: videoFileAsImageFile(f)}, nil
		}
		return &VideoFile{VideoFile: f}, nil
	case *models.ImageFile:
		return &ImageFile{ImageFile: f}, nil
	default:
		return nil, fmt.Errorf("file %s is not a visual file", f.Base().Path)
	}
}

func convertBaseFile(f models.File) BaseFile {
	if f == nil {
		return nil
	}

	switch f := f.(type) {
	case BaseFile:
		return f
	case *models.VideoFile:
		// #6949 - see convertVisualFile.
		if isAnimatedImageFile(f) {
			return &ImageFile{ImageFile: videoFileAsImageFile(f)}
		}
		return &VideoFile{VideoFile: f}
	case *models.ImageFile:
		return &ImageFile{ImageFile: f}
	case *models.BaseFile:
		// assume gallery file if it's not a video or image file
		return &GalleryFile{BaseFile: f}
	default:
		panic(fmt.Errorf("unknown file type %T", f))
	}
}

func convertBaseFiles(files []models.File) []BaseFile {
	return sliceutil.Map(files, convertBaseFile)
}

type GalleryFile struct {
	*models.BaseFile
}

func (GalleryFile) IsBaseFile() {}

func (f *GalleryFile) Fingerprints() []models.Fingerprint {
	return f.BaseFile.Fingerprints
}

type VideoFile struct {
	*models.VideoFile
}

func (VideoFile) IsBaseFile() {}

func (VideoFile) IsVisualFile() {}

func (f *VideoFile) Fingerprints() []models.Fingerprint {
	return f.VideoFile.Fingerprints
}

type ImageFile struct {
	*models.ImageFile
}

func (ImageFile) IsBaseFile() {}

func (ImageFile) IsVisualFile() {}

func (f *ImageFile) Fingerprints() []models.Fingerprint {
	return f.ImageFile.Fingerprints
}

type BasicFile struct {
	*models.BaseFile
}

func (BasicFile) IsBaseFile() {}

func (BasicFile) IsVisualFile() {}

func (f *BasicFile) Fingerprints() []models.Fingerprint {
	return f.BaseFile.Fingerprints
}

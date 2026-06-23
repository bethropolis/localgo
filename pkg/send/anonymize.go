package send

import "strings"

// anonymizeFileName maps a MIME type to a generic filename for private mode.
func anonymizeFileName(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/jpeg"):
		return "image.jpg"
	case strings.HasPrefix(contentType, "image/png"):
		return "image.png"
	case strings.HasPrefix(contentType, "image/webp"):
		return "image.webp"
	case strings.HasPrefix(contentType, "image/"):
		return "image.bin"
	case strings.HasPrefix(contentType, "video/mp4"):
		return "video.mp4"
	case strings.HasPrefix(contentType, "video/webm"):
		return "video.webm"
	case strings.HasPrefix(contentType, "video/x-matroska"):
		return "video.mkv"
	case strings.HasPrefix(contentType, "video/quicktime"):
		return "video.mov"
	case strings.HasPrefix(contentType, "video/"):
		return "video.bin"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio.mp3"
	case strings.HasPrefix(contentType, "text/plain"):
		return "document.txt"
	case strings.HasPrefix(contentType, "text/html"):
		return "document.html"
	case strings.HasPrefix(contentType, "text/"):
		return "document.txt"
	case contentType == "application/pdf":
		return "document.pdf"
	case strings.HasPrefix(contentType, "application/zip"):
		return "archive.zip"
	case strings.HasPrefix(contentType, "application/gzip"):
		return "archive.tar.gz"
	case strings.HasPrefix(contentType, "application/x-tar"):
		return "archive.tar"
	case strings.HasPrefix(contentType, "application/x-"):
		return "archive.bin"
	case strings.HasPrefix(contentType, "application/"):
		return "document.bin"
	default:
		return "file.bin"
	}
}

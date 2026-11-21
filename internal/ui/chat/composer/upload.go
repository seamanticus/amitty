package composer

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"

	"github.com/seamanticus/amitty/internal/dmwire/api"
	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/utils/sendpart"
	"github.com/seamanticus/amitty/internal/ningen"
)

// uploader puts a file's bytes at the upload URL Discord gave.
type uploader interface {
	Put(ctx context.Context, url string, body io.Reader, size int64) error
}

// sendWithFiles sends data with files as the web client does: for each file, an upload URL, then the bytes put there, then the message naming the uploads. An upload that fails sends nothing.
func sendWithFiles(ctx context.Context, state *ningen.State, put uploader, channelID discord.ChannelID, data api.SendMessageData, files []sendpart.File) error {
	uploaded := make([]api.UploadedFile, 0, len(files))
	for _, file := range files {
		size, ok := file.GuessSize()
		if !ok {
			return fmt.Errorf("upload %s: size unknown", file.Name)
		}
		upload, err := state.Client.WithContext(ctx).RequestUpload(channelID, api.UploadFile{
			Name:        file.Name,
			Size:        size,
			ContentType: contentType(file.Name),
		})
		if err != nil {
			return fmt.Errorf("upload %s: %w", file.Name, err)
		}
		if err := put.Put(ctx, upload.URL, file.Reader, size); err != nil {
			return fmt.Errorf("upload %s: %w", file.Name, err)
		}
		uploaded = append(uploaded, api.UploadedFile{Name: file.Name, Upload: upload})
	}
	if _, err := state.Client.WithContext(ctx).SendMessageUploaded(channelID, data, uploaded); err != nil {
		return fmt.Errorf("send the message: %w", err)
	}
	return nil
}

// contentType is the media type name's extension gives, without parameters, as the web client names "text/plain"; empty when unknown.
func contentType(name string) string {
	mediaType, _, err := mime.ParseMediaType(mime.TypeByExtension(filepath.Ext(name)))
	if err != nil {
		return ""
	}
	return mediaType
}

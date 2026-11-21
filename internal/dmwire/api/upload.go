package api

import (
	"strconv"

	"github.com/seamanticus/amitty/internal/dmwire/discord"
	"github.com/seamanticus/amitty/internal/dmwire/utils/httputil"
)

// UploadFile describes a file to upload, as the web client asks for its upload URL.
type UploadFile struct {
	Name string
	Size int64
	// ContentType is the type the file's name gives, or empty when unknown.
	ContentType string
}

// Upload is where to put a file before naming it in a message.
type Upload struct {
	// URL takes the file's bytes in one PUT.
	URL string `json:"upload_url"`
	// Filename names the uploaded file in the message.
	Filename string `json:"upload_filename"`
}

// RequestUpload asks for the URL to upload file to, before it is sent in channelID, as the web client does for each file.
func (c *Client) RequestUpload(channelID discord.ChannelID, file UploadFile) (Upload, error) {
	type requested struct {
		Filename            string `json:"filename"`
		FileSize            int64  `json:"file_size"`
		ID                  string `json:"id"`
		IsClip              bool   `json:"is_clip"`
		OriginalContentType string `json:"original_content_type,omitempty"`
	}
	body := struct {
		Files []requested `json:"files"`
	}{[]requested{{Filename: file.Name, FileSize: file.Size, ID: "0", OriginalContentType: file.ContentType}}}
	var resp struct {
		Attachments []Upload `json:"attachments"`
	}
	err := c.RequestJSONInto(&resp, "POST", EndpointChannels+channelID.String()+"/attachments", httputil.WithJSONBody(body))
	if err != nil {
		return Upload{}, err
	}
	if len(resp.Attachments) != 1 {
		return Upload{}, &discord.OverboundError{Count: len(resp.Attachments), Max: 1, Thing: "upload URLs for one file"}
	}
	return resp.Attachments[0], nil
}

// UploadedFile is a file put at its Upload, named Name in the message.
type UploadedFile struct {
	Name   string
	Upload Upload
}

// SendMessageUploaded posts data with files already uploaded, in the body the web client sends, fitted to the sizes NetLogs show; data.Files is not read.
func (c *Client) SendMessageUploaded(channelID discord.ChannelID, data SendMessageData, files []UploadedFile) (*discord.Message, error) {
	type attachment struct {
		ID               string `json:"id"`
		Filename         string `json:"filename"`
		UploadedFilename string `json:"uploaded_filename"`
		IsClip           bool   `json:"is_clip"`
		IsThumbnail      bool   `json:"is_thumbnail"`
	}
	attachments := make([]attachment, len(files))
	for i, file := range files {
		attachments[i] = attachment{ID: strconv.Itoa(i), Filename: file.Name, UploadedFilename: file.Upload.Filename}
	}
	if data.MobileNetworkType == "" {
		data.MobileNetworkType = "unknown"
	}
	if data.Nonce == "" {
		data.Nonce = newNonce()
	}
	body := struct {
		MobileNetworkType string                    `json:"mobile_network_type"`
		Content           string                    `json:"content"`
		Nonce             string                    `json:"nonce"`
		Type              discord.MessageType       `json:"type"`
		TTS               bool                      `json:"tts"`
		Reference         *discord.MessageReference `json:"message_reference,omitempty"`
		AllowedMentions   *AllowedMentions          `json:"allowed_mentions,omitempty"`
		Attachments       []attachment              `json:"attachments"`
	}{data.MobileNetworkType, data.Content, data.Nonce, discord.DefaultMessage, data.TTS, data.Reference, data.AllowedMentions, attachments}
	return c.RequestJSON[*discord.Message]("POST", EndpointChannels+channelID.String()+"/messages",
		httputil.WithJSONBody(body), httputil.WithHeaders(chatInputHeader()))
}

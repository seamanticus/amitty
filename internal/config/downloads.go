package config

// DownloadMode is how a download is given its place.
type DownloadMode uint8

const (
	// DownloadAsk asks where to save, starting in downloads.dir.
	DownloadAsk DownloadMode = iota
	// DownloadToDir saves straight into downloads.dir, numbering the name if it is taken.
	DownloadToDir
	downloadModeCount
)

var downloadModes = enum[DownloadMode]{{"ask", DownloadAsk}, {"dir", DownloadToDir}}

func (d *DownloadMode) UnmarshalText(text []byte) (err error) {
	*d, err = downloadModes.parse(string(text))
	return err
}

func (d DownloadMode) MarshalText() ([]byte, error) { return downloadModes.text(d) }

func (DownloadMode) choices() []string { return downloadModes.names() }

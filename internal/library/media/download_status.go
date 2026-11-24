package media

// Download statuses, matching the downloads.status CHECK constraint. They
// live here because a title's status depends on its downloads; package
// acquire moves downloads between them.
const (
	DownloadDownloading = "downloading"
	DownloadCompleted   = "completed" // data in complete_dir, not yet imported
	DownloadImported    = "imported"
	DownloadFailed      = "failed"
)

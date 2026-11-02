package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Object storage layout (spec §8):
//
//	blobs/{sha256}                                 content-addressed artifacts
//	manifests/{packageId}/{version}/manifest.json  immutable manifests
//	uploads/{uploadId}                             multipart uploads being assembled

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func ValidDigest(d string) bool { return digestRe.MatchString(d) }

// BlobKey maps "sha256:<hex>" to "blobs/<hex>".
func BlobKey(digest string) (string, error) {
	if !ValidDigest(digest) {
		return "", fmt.Errorf("invalid digest %q", digest)
	}
	return "blobs/" + strings.TrimPrefix(digest, "sha256:"), nil
}

func ManifestKey(packageID, version string) string {
	return "manifests/" + packageID + "/" + version + "/manifest.json"
}

func UploadKey(uploadID string) string { return "uploads/" + uploadID }

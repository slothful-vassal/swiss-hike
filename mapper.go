//go:build tinygo

package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strings"
)

func pageImport(externalID string, page hikePage, gpx []byte, maxPhotos int) trailImport {
	metadata := map[string]any{
		"providerCategory": "hike",
		"sourceDifficulty": page.Difficulty,
	}
	if page.DistanceMeters != nil {
		metadata["distance"] = *page.DistanceMeters
	}
	if page.ElevationGainMeters != nil {
		metadata["elevationGain"] = *page.ElevationGainMeters
	}
	if page.ElevationLossMeters != nil {
		metadata["elevationLoss"] = *page.ElevationLossMeters
	}
	if page.DurationSeconds != nil {
		metadata["duration"] = int(*page.DurationSeconds)
	}
	if page.Difficulty != "" {
		metadata["difficulty"] = page.Difficulty
	}

	return trailImport{
		Source: trailImportSource{
			Provider:   providerID,
			ExternalID: externalID,
			URL:        "https://www.schweizer-wanderwege.ch" + detailPath + url.PathEscape(externalID),
		},
		Kind:         "planned",
		Name:         page.Title,
		Description:  page.Description,
		ActivityType: "hiking",
		Track: track{
			Format:        "gpx",
			ContentBase64: base64.StdEncoding.EncodeToString(gpx),
		},
		Photos:   photos(page.Photos, maxPhotos),
		Metadata: metadata,
	}
}

func photos(pagePhotos []pagePhoto, maxPhotos int) []photo {
	if maxPhotos == 0 {
		return nil
	}
	result := make([]photo, 0, len(pagePhotos))
	for i, pagePhoto := range pagePhotos {
		if pagePhoto.URL == "" || strings.HasSuffix(strings.ToLower(pagePhoto.URL), ".gif") {
			continue
		}
		filename := imageFilename(pagePhoto.URL, i)
		result = append(result, photo{
			ExternalID:  pagePhoto.URL,
			Filename:    filename,
			ContentType: imageContentType(filename),
			Source: mediaSource{
				Type: "url",
				URL:  pagePhoto.URL,
			},
		})
		if maxPhotos > 0 && len(result) >= maxPhotos {
			break
		}
	}
	return result
}

func imageFilename(rawURL string, index int) string {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		base := path.Base(parsed.Path)
		if base != "." && base != "/" && base != "" {
			return base
		}
	}
	return fmt.Sprintf("swiss-hike-photo-%d.jpg", index+1)
}

func imageContentType(filename string) string {
	switch strings.ToLower(path.Ext(filename)) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

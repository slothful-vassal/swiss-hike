package main

import "github.com/open-wanderer/wanderer/plugins/sdk"

type listInput = sdk.ListInput
type listOutput = sdk.ListOutput
type detailInput = sdk.DetailInput
type detailOutput = sdk.DetailOutput
type refreshSessionInput = sdk.RefreshSessionInput
type refreshSessionOutput = sdk.RefreshSessionOutput
type trailSummary = sdk.TrailSummary
type trailImport = sdk.TrailImport
type trailImportSource = sdk.TrailImportSource
type track = sdk.Track
type photo = sdk.Photo
type mediaSource = sdk.MediaSource
type pluginError = sdk.PluginError

type mapFilterResponse struct {
	Hikes []string `json:"hikes"`
}

type hikePage struct {
	Title               string
	Description         string
	Photos              []pagePhoto
	GPXURL              string
	DurationSeconds     *float64
	DistanceMeters      *float64
	ElevationGainMeters *float64
	ElevationLossMeters *float64
	Difficulty          string
}

type pagePhoto struct {
	URL     string
	Caption string
}

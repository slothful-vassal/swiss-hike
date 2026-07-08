package main

import "testing"

func TestParseHikeHTMLExtractsImportFields(t *testing.T) {
	page, err := parseHikeHTML([]byte(`<!doctype html>
<html>
  <head><title>Rundwanderung &amp; Wasserfall</title></head>
  <body>
    <main class="ct-main">
      <ul class="uk-slideshow-items" uk-lightbox="animation: slide">
        <li>
          <a href="/media/cache/public_large/photo.jpg" data-caption="Gleich zu Beginn &amp; danach">
            <picture><img src="/media/cache/public_full/photo.jpg" alt="Fallback caption"></picture>
          </a>
        </li>
      </ul>
      <div class="l-metadata uk-flex">
        <div uk-tooltip="Wanderzeit"><i class="fa fa-clock"></i>4 h 5 min</div>
        <div uk-tooltip="Distanz"><i class="fa fa-arrows-left-right"></i>9,8 km</div>
        <div uk-tooltip="Körperliche Anforderung"><i></i>mittel</div>
        <div uk-tooltip="Aufstieg"><i></i>770 m</div>
        <div uk-tooltip="Abstieg"><i></i>650 m</div>
      </div>
      <section id="content_hike">
        <div class="l-hike-content">
          <div class="l-text">
            <p>Erster Abschnitt mit <strong>Aussicht</strong>.</p>
            <p>Zweiter Abschnitt &amp; Rückweg.</p>
          </div>
        </div>
      </section>
      <div class="ct-download-tile">
        <a href="/de/export/hike.gpx?download=1">GPX herunterladen</a>
      </div>
    </main>
  </body>
</html>`))
	if err != nil {
		t.Fatalf("parseHikeHTML() error = %v", err)
	}

	if page.Title != "Rundwanderung & Wasserfall" {
		t.Fatalf("Title = %q", page.Title)
	}
	if page.Description != "Erster Abschnitt mit Aussicht. Zweiter Abschnitt & Rückweg." {
		t.Fatalf("Description = %q", page.Description)
	}
	if page.GPXURL != "https://www.schweizer-wanderwege.ch/de/export/hike.gpx?download=1" {
		t.Fatalf("GPXURL = %q", page.GPXURL)
	}
	if len(page.Photos) != 1 {
		t.Fatalf("len(Photos) = %d", len(page.Photos))
	}
	if page.Photos[0].URL != "https://www.schweizer-wanderwege.ch/media/cache/public_large/photo.jpg" {
		t.Fatalf("photo URL = %q", page.Photos[0].URL)
	}
	if page.Photos[0].Caption != "Gleich zu Beginn & danach" {
		t.Fatalf("photo caption = %q", page.Photos[0].Caption)
	}
	assertFloatPointer(t, "DurationSeconds", page.DurationSeconds, 14700)
	assertFloatPointer(t, "DistanceMeters", page.DistanceMeters, 9800)
	assertFloatPointer(t, "ElevationGainMeters", page.ElevationGainMeters, 770)
	assertFloatPointer(t, "ElevationLossMeters", page.ElevationLossMeters, 650)
	if page.Difficulty != "moderate" {
		t.Fatalf("Difficulty = %q", page.Difficulty)
	}
}

func TestParseHikeHTMLRequiresMain(t *testing.T) {
	_, err := parseHikeHTML([]byte(`<html><body></body></html>`))
	if err == nil {
		t.Fatal("parseHikeHTML() error = nil")
	}
}

func assertFloatPointer(t *testing.T, name string, value *float64, want float64) {
	t.Helper()
	if value == nil {
		t.Fatalf("%s = nil", name)
	}
	if *value != want {
		t.Fatalf("%s = %v, want %v", name, *value, want)
	}
}

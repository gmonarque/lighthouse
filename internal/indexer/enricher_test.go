package indexer

import "testing"

func TestParseTitle(t *testing.T) {
	tests := []struct {
		input         string
		expectedTitle string
		expectedYear  int
	}{
		{
			"Avatar.2009.1080p.BluRay.x264",
			"Avatar",
			2009,
		},
		{
			"The.Matrix.1999.REMASTERED.2160p.UHD.BluRay",
			"The Matrix",
			1999,
		},
		{
			"Inception.2010.1080p.BluRay",
			"Inception",
			2010,
		},
		{
			"Breaking.Bad.S01E01.720p.HDTV",
			"Breaking Bad S01E01",
			0,
		},
		{
			"Dune.Part.Two.2024.2160p.WEB-DL.DDP5.1.Atmos.DV.HDR.H.265",
			"Dune Part Two",
			2024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			title, year := parseTitle(tt.input)
			if title != tt.expectedTitle {
				t.Errorf("parseTitle(%q) title = %q, want %q", tt.input, title, tt.expectedTitle)
			}
			if year != tt.expectedYear {
				t.Errorf("parseTitle(%q) year = %d, want %d", tt.input, year, tt.expectedYear)
			}
		})
	}
}

func TestCleanTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Movie.Name.2024", "Movie Name 2024"},
		{"Movie_Name_2024", "Movie Name 2024"},
		{"Movie-Name-2024", "Movie Name 2024"},
		{"Movie.1080p.BluRay", "Movie"},
		{"Movie.x264.DTS", "Movie"},
		{"Movie [Group]", "Movie"},
		{"Movie (Repack)", "Movie"},
		{"Movie   Extra   Spaces", "Movie Extra Spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := cleanTitle(tt.input)
			if result != tt.expected {
				t.Errorf("cleanTitle(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
